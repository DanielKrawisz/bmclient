// Copyright (c) 2015 Monetas.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package email

import (
	"bytes"
	"container/list"
	"errors"
	"sync"
	"time"

	"github.com/jordwest/imap-server/mailstore"
	"github.com/jordwest/imap-server/types"
	"github.com/mailhog/data"
	"github.com/monetas/bmclient/store"
)

// GetSequenceNumber gets the sequence number higher than or equal to the given
// uid.
func GetSequenceNumber(uids []uint64, uid uint64) uint32 {
	// TODO make the use of this redundant so that complexity goes down from
	// O(n^2) while fetching messages.

	// If the slice is empty.
	if len(uids) == 0 {
		return 0
	}

	for i, u := range uids {
		if u > uid { // We already exceeded so return the next element.
			return uint32(i + 1)
		}
		if uid == u {
			return uint32(i + 1)
		}
	}
	return 0
}

// Mailbox implements a mailbox that is compatible with IMAP. It implements the
// email.IMAPMailbox interface. Only functions that implement IMAPMailbox take
// care of locking/unlocking the embedded RWMutex.
type Mailbox struct {
	mbox         *store.Mailbox
	sync.RWMutex // Protect the following fields.
	uids         []uint64
	numRecent    uint32
	numUnseen    uint32
	nextUID      uint32
	lastUID      uint32
}

func (box *Mailbox) decodeBitmessageForImap(uid uint64, seqno uint32, msg []byte) *Bitmessage {
	b, err := DecodeBitmessage(msg)
	if b == nil {
		imapLog.Errorf("DecodeBitmessage for #%d failed: %v", uid, err)
		return nil
	}
	b.ImapData.UID = uid
	b.ImapData.SequenceNumber = seqno
	b.ImapData.Mailbox = box
	return b
}

// Name returns the name of the mailbox.
// This is part of the email.ImapFolder interface.
func (box *Mailbox) Name() string {
	return box.mbox.Name()
}

// Refresh updates cached statistics like number of messages in inbox,
// next UID, last UID, number of recent/unread messages etc. It is meant to
// be called after the mailbox has been modified by an agent other than the
// IMAP server. This could be the SMTP server, or new message from bmd.
func (box *Mailbox) Refresh() error {
	box.Lock()
	defer box.Unlock()

	var err error

	// Set NextUID
	nextUID, err := box.mbox.NextID()
	if err != nil {
		return err
	}
	box.nextUID = uint32(nextUID)

	// Set LastUID
	lastUID, err := box.mbox.LastIDBySuffix(2)
	if err == store.ErrNotFound {
		lastUID = uint64(box.nextUID)
	} else if err != nil {
		return err
	}
	box.lastUID = uint32(lastUID)

	var recent, unseen uint32
	list := list.New()

	// Run through every message to get the uids and count the recent and
	// unseen messages.
	err = box.mbox.ForEachMessage(0, 0, 2, func(id, suffix uint64, msg []byte) error {
		entry, err := DecodeBitmessage(msg)
		if err != nil {
			return imapLog.Errorf("Failed to decode message #%d: %v", id, err)
		}

		if entry.ImapData.Flags.HasFlags(types.FlagRecent) {
			recent++
		}
		if !entry.ImapData.Flags.HasFlags(types.FlagSeen) {
			unseen++
		}

		list.PushBack(id)
		return nil
	})
	if err != nil {
		return err
	}

	box.uids = make([]uint64, 0, list.Len())
	box.numRecent = recent
	box.numUnseen = unseen

	for e := list.Front(); e != nil; e = e.Next() {
		box.uids = append(box.uids, e.Value.(uint64))
	}

	return nil
}

// NextUID returns the unique identifier that will LIKELY be assigned
// to the next mail that is added to this mailbox.
// This is part of the email.ImapFolder interface.
func (box *Mailbox) NextUID() uint32 {
	box.RLock()
	defer box.RUnlock()

	return box.nextUID
}

// LastUID assigns the UID of the very last message in the mailbox.
// If the mailbox is empty, this should return the next expected UID.
// This is part of the email.ImapFolder interface.
func (box *Mailbox) LastUID() uint32 {
	box.RLock()
	defer box.RUnlock()

	return box.lastUID
}

// Recent returns the number of recent messages in the mailbox.
// This is part of the email.ImapFolder interface.
func (box *Mailbox) Recent() uint32 {
	box.RLock()
	defer box.RUnlock()

	return box.numRecent
}

// Messages returns the number of messages in the mailbox.
// This is part of the email.ImapFolder interface.
func (box *Mailbox) Messages() uint32 {
	box.RLock()
	defer box.RUnlock()

	return box.messages()
}

// messages returns the number of messages in the mailbox. It doesn't use the
// RWLock.
func (box *Mailbox) messages() uint32 {
	return uint32(len(box.uids))
}

// Unseen returns the number of messages that do not have the Unseen flag set yet
// This is part of the email.ImapFolder interface.
func (box *Mailbox) Unseen() uint32 {
	box.RLock()
	defer box.RUnlock()

	return box.numUnseen
}

// BitmessageBySequenceNumber gets a message by its sequence number
func (box *Mailbox) BitmessageBySequenceNumber(seqno uint32) *Bitmessage {
	if seqno < 1 || seqno > box.messages() {
		return nil
	}
	uid := box.uids[seqno-1]
	return box.bmsgByUID(uid)
}

// MessageBySequenceNumber gets a message by its sequence number
// It is a part of the mail.SMTPFolder interface.
func (box *Mailbox) MessageBySequenceNumber(seqno uint32) mailstore.Message {
	box.RLock()
	defer box.RUnlock()

	bm := box.BitmessageBySequenceNumber(seqno)
	if bm == nil {
		return nil
	}
	email, err := bm.ToEmail()
	if err != nil {
		imapLog.Error("MessageBySequenceNumber (%d) gave error %v", seqno, err)
		return nil
	}

	return email
}

// bmsgByUID returns a Bitmessage by its uid. It's not protected with locks.
func (box *Mailbox) bmsgByUID(uid uint64) *Bitmessage {
	suffix, msg, err := box.mbox.GetMessage(uid)
	if err != nil {
		imapLog.Errorf("Mailbox(%s).GetMessage gave error: %v", box.Name(), err)
		return nil
	}
	if suffix != 2 {
		imapLog.Errorf("For message #%d expected suffix %d got %d", uid, 2, suffix)
		return nil
	}

	seqno := GetSequenceNumber(box.uids, uint64(uid))

	return box.decodeBitmessageForImap(uid, seqno, msg)
}

// BitmessageByUID returns a Bitmessage by its uid.
func (box *Mailbox) BitmessageByUID(uid uint64) *Bitmessage {
	return box.bmsgByUID(uid)
}

// MessageByUID gets a message by its uid number
// It is a part of the mail.SMTPFolder interface.
func (box *Mailbox) MessageByUID(uid uint32) mailstore.Message {
	box.RLock()
	defer box.RUnlock()

	letter := box.BitmessageByUID(uint64(uid))
	if letter == nil {
		return nil
	}
	email, err := letter.ToEmail()
	if err != nil {
		imapLog.Errorf("Failed to convert message #%d to e-mail: %v", uid, err)
	}
	return email
}

// LastBitmessage returns the last Bitmessage in the mailbox.
func (box *Mailbox) LastBitmessage() *Bitmessage {
	if box.messages() == 0 {
		return nil
	}

	uid := box.uids[len(box.uids)-1]
	return box.bmsgByUID(uid)
}

// getRange returns a sequence of bitmessages from the mailbox in a range from
// startUID to endUID. It does not check whether the given sequence numbers make
// sense.
func (box *Mailbox) getRange(startUID, endUID uint64, startSequence, endSequence uint32) []*Bitmessage {
	bitmessages := make([]*Bitmessage, 0, endSequence-startSequence+1)

	i := uint32(0)
	err := box.mbox.ForEachMessage(startUID, endUID, 2, func(id, suffix uint64, msg []byte) error {
		bm := box.decodeBitmessageForImap(id, startSequence+i, msg)
		if bm == nil {
			return nil // Skip this message, error has already been logged.
		}
		bitmessages = append(bitmessages, bm)
		i++
		return nil
	})
	if err != nil {
		return nil
	}
	return bitmessages
}

// getSince returns a sequence of bitmessages from the mailbox which includes
// all greater than or equal to a given uid number. It does not check whether
// the given sequence number makes sense.
func (box *Mailbox) getSince(startUID uint64, startSequence uint32) []*Bitmessage {
	return box.getRange(startUID, 0, startSequence, box.messages())
}

// BitmessagesByUIDRange returns the last Bitmessage in the mailbox.
func (box *Mailbox) BitmessagesByUIDRange(start, end uint64) []*Bitmessage {
	startSequence := GetSequenceNumber(box.uids, start)
	endSequence := GetSequenceNumber(box.uids, end)
	if endSequence == 0 { // We exceeded the range
		endSequence = box.messages()
	}

	if startSequence > endSequence {
		return []*Bitmessage{}
	}
	return box.getRange(start, end, startSequence, endSequence)
}

// BitmessagesSinceUID returns the last Bitmessage in the mailbox.
func (box *Mailbox) BitmessagesSinceUID(start uint64) []*Bitmessage {
	startSequence := GetSequenceNumber(box.uids, start)
	return box.getSince(start, startSequence)
}

// BitmessagesBySequenceRange returns a set of Bitmessages in a range between two sequence numbers inclusive.
func (box *Mailbox) BitmessagesBySequenceRange(start, end uint32) []*Bitmessage {
	if start < 1 || start > box.messages() ||
		end < 1 || end > box.messages() || end < start {
		return nil
	}
	startUID := box.uids[start]
	endUID := box.uids[end]
	return box.getRange(startUID, endUID, start, end)
}

// BitmessagesSinceSequenceNumber returns the set of Bitmessages since and including a given uid value.
func (box *Mailbox) BitmessagesSinceSequenceNumber(start uint32) []*Bitmessage {
	if start < 1 || start > box.Messages() {
		return nil
	}
	startUID := box.uids[start]
	return box.getSince(startUID, start)
}

// BitmessageSetByUID gets messages belonging to a set of ranges of UIDs
func (box *Mailbox) BitmessageSetByUID(set types.SequenceSet) []*Bitmessage {
	// TODO review and fix
	var msgs []*Bitmessage

	// If the mailbox is empty, return empty array
	if box.messages() == 0 {
		return msgs
	}

	for _, msgRange := range set {
		// If Min is "*", meaning the last UID in the mailbox, Max should
		// always be Nil
		if msgRange.Min.Last() {
			// Return the last message in the mailbox
			msgs = append(msgs, box.LastBitmessage())
			continue
		}

		start, err := msgRange.Min.Value()
		if err != nil {
			return msgs
		}

		// If no Max is specified, then return only the min value.
		if msgRange.Max.Nil() {
			// Fetch specific message by sequence number
			msgs = append(msgs, box.BitmessageByUID(uint64(start)))
			if err != nil {
				return msgs
			}
			continue
		}

		var end uint32
		if msgRange.Max.Last() {
			since := box.BitmessagesSinceUID(uint64(start))
			if since == nil {
				continue // Some error occurred
			}
			msgs = append(msgs, since...)
		} else {
			end, err = msgRange.Max.Value()
			if err != nil {
				return msgs
			}
			msgs = append(msgs, box.BitmessagesByUIDRange(uint64(start), uint64(end))...)
		}
	}
	return msgs
}

// BitmessageSetBySequenceNumber gets messages belonging to a set of ranges of sequence numbers
func (box *Mailbox) BitmessageSetBySequenceNumber(set types.SequenceSet) []*Bitmessage {
	var msgs []*Bitmessage

	// If the mailbox is empty, return empty array
	if box.Messages() == 0 {
		return msgs
	}

	for _, msgRange := range set {
		// If Min is "*", meaning the last UID in the mailbox, Max should
		// always be Nil
		if msgRange.Min.Last() {
			// Return the last message in the mailbox
			msgs = append(msgs, box.LastBitmessage())
			continue
		}

		startIndex, err := msgRange.Min.Value()
		if err != nil {
			return msgs
		}
		if startIndex < 1 || startIndex > box.Messages() {
			return msgs
		}
		start := uint32(box.uids[startIndex-1])

		// If no Max is specified, then return only the min value.
		if msgRange.Max.Nil() {
			// Fetch specific message by sequence number
			msgs = append(msgs, box.BitmessageBySequenceNumber(start))
			if err != nil {
				return msgs
			}
			continue
		}

		var end uint32
		if msgRange.Max.Last() {
			msgs = append(msgs, box.BitmessagesSinceSequenceNumber(start)...)
		} else {
			end, err = msgRange.Max.Value()
			if err != nil {
				return msgs
			}
			msgs = append(msgs, box.BitmessagesBySequenceRange(start, end)...)
		}
	}

	return msgs
}

// AddNew adds a new Bitmessage to the Mailbox.
func (box *Mailbox) AddNew(bmsg *Bitmessage, flags types.Flags) error {
	encoding := bmsg.Payload.Encoding()
	if encoding != 2 {
		return errors.New("Unsupported encoding")
	}

	imapData := &IMAPData{
		SequenceNumber: box.messages(),
		Flags:          flags,
		DateReceived:   time.Now(),
		Mailbox:        box,
	}

	bmsg.ImapData = imapData

	msg, err := bmsg.Serialize()
	if err != nil {
		return err
	}

	uid, err := box.mbox.InsertMessage(msg, 0, bmsg.Payload.Encoding())
	if err != nil {
		return err
	}

	imapData.UID = uid
	return box.Refresh()
}

// MessageSetByUID returns the slice of messages belonging to a set of ranges of
// UIDs.
// It is a part of the mail.SMTPFolder interface.
func (box *Mailbox) MessageSetByUID(set types.SequenceSet) []mailstore.Message {
	box.RLock()
	defer box.RUnlock()
	var err error

	msgs := box.BitmessageSetByUID(set)
	email := make([]mailstore.Message, len(msgs))
	for i, msg := range msgs {
		email[i], err = msg.ToEmail()
		if err != nil {
			imapLog.Errorf("Failed to convert message #%d to e-mail: %v",
				msg.ImapData.UID, err)
			return nil
		}
	}
	return email
}

// MessageSetBySequenceNumber returns the slice of messages belonging to a set
// of ranges of sequence numbers.
// It is a part of the mail.SMTPFolder interface.
func (box *Mailbox) MessageSetBySequenceNumber(set types.SequenceSet) []mailstore.Message {
	box.RLock()
	defer box.RUnlock()
	var err error

	msgs := box.BitmessageSetBySequenceNumber(set)
	email := make([]mailstore.Message, len(msgs))
	for i, msg := range msgs {
		email[i], err = msg.ToEmail()
		if err != nil {
			imapLog.Errorf("Failed to convert message #%d to e-mail: %v",
				msg.ImapData.UID, err)
			return nil
		}
	}
	return email
}

// Save saves the given bitmessage entry in the folder.
func (box *Mailbox) SaveBitmessage(msg *Bitmessage) error {
	if msg.ImapData.UID != 0 { // The message already exists and needs to be replaced.
		// Check that the uid, date, and sequence number are consistent with one another.
		previous := box.BitmessageByUID(msg.ImapData.UID)
		if previous == nil {
			return errors.New("Invalid sequence number")
		}
		if previous.ImapData.UID != msg.ImapData.UID {
			return errors.New("Invalid uid")
		}
		if previous.ImapData.DateReceived != msg.ImapData.DateReceived {
			return errors.New("Cannot change date received")
		}

		// Delete the old message from the database.
		err := box.mbox.DeleteMessage(uint64(msg.ImapData.UID))
		if err != nil {
			imapLog.Errorf("Mailbox(%s).DeleteMessage(%d) gave error %v",
				box.Name(), msg.ImapData.UID, err)
			return err
		}
	}

	// Generate the new version of the message.
	encode, err := msg.Serialize()
	if err != nil {
		return err
	}

	// Insert the new version of the message.
	newUID, err := box.mbox.InsertMessage(encode, msg.ImapData.UID, msg.Payload.Encoding())
	if err != nil {
		imapLog.Errorf("Mailbox(%s).InsertMessage(id=%d, suffix=%d) gave error %v",
			box.Name(), msg.ImapData.UID, msg.Payload.Encoding())
		return err
	}

	msg.ImapData.UID = newUID

	err = box.Refresh()
	if err != nil {
		imapLog.Errorf("Mailbox(%s).Refresh gave error %v", box.Name(), err)
		return err
	}
	return nil
}

// Save saves an IMAP email in the Mailbox. It is part of the IMAPMailbox
// interface.
func (box *Mailbox) Save(email *IMAPEmail) error {
	bm, err := NewBitmessageFromSMTP(email.Content)
	if err != nil {
		imapLog.Errorf("Error saving message #%d: %v", email.ImapUID, err)
		return err
	}

	bm.ImapData = &IMAPData{
		UID:            email.ImapUID,
		SequenceNumber: email.ImapSequenceNumber,
		Flags:          email.ImapFlags,
		DateReceived:   email.Date,
		Mailbox:        box,
	}

	return box.SaveBitmessage(bm)
}

// This error is used to cause mailbox.ForEachMessage to stop looping through
// every message once an ack is found, but is not a real error.
var errAckFound = errors.New("Ack Found")

// ReceiveAck takes an object payload and tests it against messages in the
// folder to see if it matches the ack of any sent message in the folder.
// The first such message found is returned.
func (box *Mailbox) ReceiveAck(ack []byte) *Bitmessage {
	var ackMatch *Bitmessage

	box.mbox.ForEachMessage(0, 0, 2, func(id, suffix uint64, msg []byte) error {
		entry, err := DecodeBitmessage(msg)
		if err != nil {
			return err
		}

		if bytes.Equal(entry.Ack, ack) {
			ackMatch = entry

			// Stop ForEachMessage from searching the rest of the messages.
			return errAckFound
		}
		return nil
	})
	if ackMatch == nil {
		return nil
	}

	ackMatch.AckReceived = true
	box.SaveBitmessage(ackMatch)

	return ackMatch
}

// NewMessage creates a new empty message associated with this folder.
// It is part of the IMAPMailbox interface.
func (box *Mailbox) NewMessage() mailstore.Message {
	return &IMAPEmail{
		ImapFlags: types.FlagRecent,
		Mailbox:   box,
		Content:   &data.Content{},
	}
}

// NewMailbox returns a new mailbox.
func NewMailbox(mbox *store.Mailbox) (*Mailbox, error) {
	m := &Mailbox{
		mbox: mbox,
	}

	// Populate various data fields.
	if err := m.Refresh(); err != nil {
		return nil, err
	}
	return m, nil
}
