// Copyright (c) 2015 Monetas.
// Copyright 2016 Daniel Krawisz.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"

	"github.com/DanielKrawisz/bmagent/keymgr/keys"
	"github.com/DanielKrawisz/bmagent/store"
	"github.com/DanielKrawisz/bmagent/user/command"
	"github.com/DanielKrawisz/bmagent/user/email"
	"github.com/DanielKrawisz/bmutil/format"
	"github.com/jordwest/imap-server/mailstore"
	"github.com/jordwest/imap-server/types"
)

// BitmessageStore implements mailstore.Mailstore.
type BitmessageStore struct {
	cfg  *email.IMAPConfig
	user *User
}

// Authenticate is part of the mailstore.Mailstore interface. It takes
// a username and password and returns a mailstore.User if the credentials
// are valid.
func (s *BitmessageStore) Authenticate(username string, password string) (mailstore.User, error) {
	email.IMAPLog.Tracef("imap authentication attempt with u=%s, p=%s", username, password)

	// TODO Use constant time comparisons.
	if username != s.cfg.Username || password != s.cfg.Password {
		return nil, errors.New("Invalid credentials")
	}

	return s.user, nil
}

// Initialize initializes the store by creating the default mailboxes and
// inserting the welcome message.
func Initialize(u *store.UserData, k keys.Manager, genkeys uint32) error {
	// Get all keys from key manager.
	tags := k.Names()

	// Create Inbox.
	mbox, err := u.NewFolder(InboxFolderName)
	if err != nil {
		return err
	}
	inbox, err := newMailbox(mbox, tags)
	if err != nil {
		return err
	}

	_, err = u.NewFolder(OutboxFolderName)
	if err != nil {
		return err
	}
	_, err = u.NewFolder(SentFolderName)
	if err != nil {
		return err
	}
	_, err = u.NewFolder(LimboFolderName)
	if err != nil {
		return err
	}
	_, err = u.NewFolder(TrashFolderName)
	if err != nil {
		return err
	}
	_, err = u.NewFolder(CommandsFolderName)
	if err != nil {
		return err
	}
	_, err = u.NewFolder(DraftsFolderName)
	if err != nil {
		return err
	}

	var i uint32
	for i = 0; i < genkeys; i++ {
		k.NewUnnamed(command.DefaultStream, command.DefaultBehavior)
	}

	// For each key, create a mailbox.
	var toAddr string
	keyList := ""

	for addr, tag := range tags {
		keyList = fmt.Sprint(keyList, fmt.Sprintf("\t%s@bm.addr %s\n", addr, tag))
	}

	welcome := fmt.Sprintf(welcomeMsg, keyList)

	// Add the introductory message.
	from := "welcome@bm.agent"
	subject := "Welcome to bmagent!"

	err = inbox.AddNew(&email.Bmail{
		From: from,
		To:   fmt.Sprintf("%s@bm.addr", toAddr),
		Content: &format.Encoding2{
			Subject: subject,
			Body:    welcome,
		},
	}, types.FlagRecent)
	if err != nil {
		return err
	}

	return nil
}

// NewBitmessageStore creates a new bitmessage store.
func NewBitmessageStore(user *User, cfg *email.IMAPConfig) *BitmessageStore {
	return &BitmessageStore{
		user: user,
		cfg:  cfg,
	}
}
