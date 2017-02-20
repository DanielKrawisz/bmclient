package store_test

import (
	"errors"
	"testing"

	"github.com/DanielKrawisz/bmagent/store/data"
)

type message struct {
	payload []byte
	suffix  uint64
}

// folder that exists for testing purposes.
type testFolder struct {
	name      string
	nextIndex uint64
	messages  map[uint64]message
}

func (f *testFolder) Name() string {
	return f.name
}

func (f *testFolder) SetName(name string) error {
	f.name = name
	return nil
}

func (f *testFolder) NextID() uint64 {
	return f.nextIndex
}

func (f *testFolder) LastID() (uint64, map[uint64]uint64) {
	lastBySuffix := make(map[uint64]uint64)
	var lastId uint64 = 0
	var id uint64

	for id = 1; id < f.nextIndex; id++ {
		m, ok := f.messages[id]

		if !ok {
			continue
		}

		lastId = id
		lastBySuffix[m.suffix] = id
	}

	return lastId, lastBySuffix
}

func (f *testFolder) InsertNewMessage(msg []byte, suffix uint64) (uint64, error) {
	if msg == nil {
		return 0, errors.New("Nil message inserted.")
	}

	defer func() {
		f.nextIndex++
	}()

	f.messages[f.nextIndex] = message{payload: msg, suffix: suffix}

	return f.nextIndex, nil
}

func (f *testFolder) InsertMessage(id uint64, msg []byte, suffix uint64) error {
	if id == 0 || id >= f.nextIndex {
		return data.ErrInvalidID
	}

	if msg == nil {
		return errors.New("Nil message inserted.")
	}

	if _, ok := f.messages[id]; ok {
		return data.ErrDuplicateID
	}

	f.messages[id] = message{payload: msg, suffix: suffix}

	return nil
}

func (f *testFolder) GetMessage(id uint64) (uint64, []byte, error) {
	if id == 0 || id >= f.nextIndex {
		return 0, nil, data.ErrNotFound
	}

	if m, ok := f.messages[id]; !ok {
		return 0, nil, data.ErrNotFound
	} else {
		return m.suffix, m.payload, nil
	}
}

func (f *testFolder) DeleteMessage(id uint64) error {
	if id == 0 || id >= f.nextIndex {
		return data.ErrInvalidID
	}

	if _, ok := f.messages[id]; !ok {
		return data.ErrNotFound
	}

	delete(f.messages, id)

	return nil
}

func (f *testFolder) ForEachMessage(lowID, highID, suffix uint64,
	fn func(id, suffix uint64, msg []byte) error) error {

	for id, m := range f.messages {
		err := fn(id, m.suffix, m.payload)

		if err != nil {
			return err
		}
	}

	return nil
}

// testContext is used to store context information about a running test which
// is passed into helper functions.
// In the future, this could be expanded to encompass cases relevant to
// bolt, in which the folder must be closed re-opened. Right now that case
// is tested in an ad-hoc way.
type testContext interface {
	T() *testing.T
	GetFolder(string) data.Folder
	Context() string
}

type testFolderTestContext struct {
	t *testing.T
}

func (tc *testFolderTestContext) T() *testing.T {
	return tc.t
}

func (tc *testFolderTestContext) GetFolder(name string) data.Folder {
	return &testFolder{
		name:      name,
		nextIndex: 1,
		messages:  make(map[uint64]message),
	}
}

func (tc *testFolderTestContext) Context() string {
	return "test"
}

type memFolderTestContext struct {
	t *testing.T
	f data.Folders
}

func (tc *memFolderTestContext) T() *testing.T {
	return tc.t
}

func (tc *memFolderTestContext) GetFolder(name string) data.Folder {
	folder, err := tc.f.Get(name)
	if err == nil {
		return folder
	}
	folder, _ = tc.f.New(name)
	return folder
}

func (tc *memFolderTestContext) Context() string {
	return "mem"
}

type boltFolderTestContext struct {
	t       *testing.T
	name    string
	folders data.Folders
}

func (tc *boltFolderTestContext) T() *testing.T {
	return tc.t
}

func (tc *boltFolderTestContext) GetFolder(name string) data.Folder {
	f, err := tc.folders.Get(name)
	if err == nil {
		return f
	}

	f, _ = tc.folders.New(name)
	return f
}

func (tc *boltFolderTestContext) Context() string {
	return "bolt"
}

func TestInterface(t *testing.T) {

	var folders []testContext = make([]testContext, 3)

	// Create test folder context (used only for comparison to the other folders).
	folders[0] = &testFolderTestContext{t: t}

	// Create mem folder test context
	folders[1] = &memFolderTestContext{t: t, f: data.NewMemFolders()}

	// Create bolt folder test context
	f, _ := NewUser(t).Folders()
	folders[2] = &boltFolderTestContext{t: t, folders: f, name: "subscriptions"}

	for _, tc := range folders {
		testInterface(tc)
	}
}

func testInterface(tc testContext) {

	// new folder should be empty.
	folder := tc.GetFolder("test folder")
	if folder == nil {
		tc.T().Fail()
		return
	}

	testNextLast(tc, folder, 0, 1, "0")

	testInvalidIndex(tc, folder, 1)

	// Try to add a new item to the mailbox.
	testInsertMessage(folder, []byte("It's a new message."), 1, 1, tc.T())

	testNextLast(tc, folder, 1, 2, "A")

	testInsertMessage(folder, []byte("Another new message."), 2, 2, tc.T())

	testNextLast(tc, folder, 2, 3, "B")

	testLastIDBySuffix(tc, folder, 2, 2, "C")
	testLastIDBySuffix(tc, folder, 1, 1, "D")
	testLastIDBySuffix(tc, folder, 0, 0, "E")
	testLastIDBySuffix(tc, folder, 3, 0, "F")

	testInsertMessage(folder, []byte("Entry 3"), 3, 3, tc.T())

	testLastIDBySuffix(tc, folder, 2, 2, "G")
	testLastIDBySuffix(tc, folder, 3, 3, "H")

	testInsertMessage(folder, []byte("Entry 4"), 2, 4, tc.T())
	testInsertMessage(folder, []byte("My diary is so boring."), 1, 5, tc.T())
	testInsertMessage(folder, []byte("Mooooo"), 3, 6, tc.T())

	testLastIDBySuffix(tc, folder, 2, 4, "I")
	testLastIDBySuffix(tc, folder, 1, 5, "J")
	testLastIDBySuffix(tc, folder, 3, 6, "K")
	testNextLast(tc, folder, 6, 7, "L")

	testDeleteMessage(tc, folder, 4, "M")
	testLastIDBySuffix(tc, folder, 2, 2, "N")
	testNextLast(tc, folder, 6, 7, "O")

	testDeleteMessage(tc, folder, 3, "P")
	testLastIDBySuffix(tc, folder, 3, 6, "Q")
	testNextLast(tc, folder, 6, 7, "R")

	testDeleteMessage(tc, folder, 6, "S")
	testLastIDBySuffix(tc, folder, 3, 0, "T")
	testNextLast(tc, folder, 5, 7, "U")
}

func testNextLast(tc testContext, folder data.Folder, exLast, exNext uint64, note string) {

	last, _ := folder.LastID()
	next := folder.NextID()

	if last != exLast {
		tc.T().Error(tc.Context(), ", ", note, ": new folder's last id should be ", exLast, " got ", last)
	}

	if next != exNext {
		tc.T().Error(tc.Context(), ", ", note, ": new folder's next id should be ", exNext, " got ", next)
	}

}

// Test that err invalid index is returned
func testInvalidIndex(tc testContext, folder data.Folder, max uint64) {
	if err := folder.InsertMessage(0, nil, 0); err != data.ErrInvalidID {
		tc.T().Error("Expected ", data.ErrInvalidID, " got ", err)
	}

	if err := folder.InsertMessage(max, nil, 0); err != data.ErrInvalidID {
		tc.T().Error("Expected ", data.ErrInvalidID, " got ", err)
	}

	if _, _, err := folder.GetMessage(0); err != data.ErrNotFound {
		tc.T().Error("Expected ", data.ErrNotFound, " got ", err)
	}

	if _, _, err := folder.GetMessage(max); err != data.ErrNotFound {
		tc.T().Error("Expected ", data.ErrNotFound, " got ", err)
	}

	if err := folder.DeleteMessage(0); err != data.ErrInvalidID {
		tc.T().Error("Expected ", data.ErrInvalidID, " got ", err)
	}

	if err := folder.DeleteMessage(max); err != data.ErrInvalidID {
		tc.T().Error("Expected ", data.ErrInvalidID, " got ", err)
	}
}
