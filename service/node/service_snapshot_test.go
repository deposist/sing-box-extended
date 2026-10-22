package node

import (
	"testing"

	"github.com/sagernet/sing-box/adapter"
	CM "github.com/sagernet/sing-box/service/manager/constant"
	"github.com/sagernet/sing-box/service/node/constant"
)

// recordingUserManager captures the user sets a per-inbound manager sees.
type recordingUserManager struct {
	updates [][]CM.User
}

func (m *recordingUserManager) UpdateUser(user CM.User) {
	m.updates = append(m.updates, []CM.User{user})
}

func (m *recordingUserManager) UpdateUsers(users []CM.User) {
	m.updates = append(m.updates, append([]CM.User(nil), users...))
}

func (m *recordingUserManager) DeleteUser(username string) {}

type recordingInboundManager struct {
	managers map[string]*recordingUserManager
	tags     []string
}

func (m *recordingInboundManager) AddUserManager(inbound adapter.Inbound) error { return nil }

func (m *recordingInboundManager) GetUserManager(tag string) (constant.UserManager, bool) {
	manager, ok := m.managers[tag]
	return manager, ok
}

func (m *recordingInboundManager) GetUserManagerTags() []string { return m.tags }

func newRecordingService(inboundsByType map[string][]string) (*Service, map[string]*recordingUserManager) {
	service := &Service{inboundManagers: make(map[string]constant.InboundManager)}
	userManagers := make(map[string]*recordingUserManager)
	for type_, tags := range inboundsByType {
		manager := &recordingInboundManager{managers: make(map[string]*recordingUserManager)}
		for _, tag := range tags {
			userManager := &recordingUserManager{}
			manager.managers[tag] = userManager
			manager.tags = append(manager.tags, tag)
			userManagers[tag] = userManager
		}
		service.inboundManagers[type_] = manager
	}
	return service, userManagers
}

// TestUpdateUsersGroupsByInboundNotByType guards the snapshot contract: a user
// assigned to one inbound must never be pushed into a sibling inbound of the
// same protocol, and an inbound absent from the snapshot must end up empty.
func TestUpdateUsersGroupsByInboundNotByType(t *testing.T) {
	service, userManagers := newRecordingService(map[string][]string{
		"vless": {"vless-a", "vless-b"},
	})

	service.UpdateUsers([]CM.User{
		{Username: "alice", Type: "vless", Inbound: "vless-a"},
		{Username: "bob", Type: "vless", Inbound: "vless-a"},
		{Username: "carol", Type: "vless", Inbound: "vless-b"},
	})

	inboundA := userManagers["vless-a"].updates
	if len(inboundA) != 1 {
		t.Fatalf("vless-a should receive exactly one snapshot, got %d", len(inboundA))
	}
	if len(inboundA[0]) != 2 {
		t.Fatalf("vless-a should receive its two users, got %d", len(inboundA[0]))
	}

	inboundB := userManagers["vless-b"].updates
	if len(inboundB) != 1 {
		t.Fatalf("vless-b should receive exactly one snapshot, got %d", len(inboundB))
	}
	if len(inboundB[0]) != 1 || inboundB[0][0].Username != "carol" {
		t.Fatalf("vless-b should receive only carol, got %+v", inboundB[0])
	}
}

// TestUpdateUsersClearsInboundAbsentFromSnapshot is the offline-delete case:
// the manager dropped every user of an inbound while the node was away, so the
// snapshot omits that inbound entirely and the node must empty it.
func TestUpdateUsersClearsInboundAbsentFromSnapshot(t *testing.T) {
	service, userManagers := newRecordingService(map[string][]string{
		"vless": {"vless-a", "vless-b"},
	})
	service.UpdateUsers([]CM.User{
		{Username: "alice", Type: "vless", Inbound: "vless-a"},
	})

	updates := userManagers["vless-b"].updates
	if len(updates) != 1 {
		t.Fatalf("vless-b should be reset exactly once, got %d updates", len(updates))
	}
	if len(updates[0]) != 0 {
		t.Fatalf("vless-b should be emptied, got %+v", updates[0])
	}
}

// TestUpdateUsersEmptySnapshotEmptiesEverything covers revoking the last user.
func TestUpdateUsersEmptySnapshotEmptiesEverything(t *testing.T) {
	service, userManagers := newRecordingService(map[string][]string{
		"vless":  {"vless-a"},
		"trojan": {"trojan-a"},
	})
	service.UpdateUsers(nil)

	for _, tag := range []string{"vless-a", "trojan-a"} {
		updates := userManagers[tag].updates
		if len(updates) != 1 || len(updates[0]) != 0 {
			t.Fatalf("%s should be emptied by an empty snapshot, got %+v", tag, updates)
		}
	}
}
