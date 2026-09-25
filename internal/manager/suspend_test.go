// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package manager

import (
	"testing"
	"time"

	"github.com/Bahonio/awg-docui/internal/api"
)

// The suspender suspends exactly the active clients whose time has passed.
func TestSuspendDueSuspendsOnlyOverdueActiveClients(t *testing.T) {
	m, _ := newTestManager(t)
	past := float64(time.Now().Add(-time.Minute).Unix())
	future := float64(time.Now().Add(time.Hour).Unix())

	overdue, _, _ := m.AddClient("s1", api.AddClientRequest{Name: "overdue"})
	later, _, _ := m.AddClient("s1", api.AddClientRequest{Name: "later"})
	already, _, _ := m.AddClient("s1", api.AddClientRequest{Name: "already"})
	for _, c := range []struct {
		id string
		at *float64
	}{{overdue.ID, &past}, {later.ID, &future}, {already.ID, &past}} {
		if _, _, err := m.UpdateClientSuspendTime("s1", c.id, c.at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.SuspendClient("s1", already.ID); err != nil {
		t.Fatal(err)
	}

	m.suspendDue(time.Now())

	want := map[string]string{overdue.ID: "suspended", later.ID: "active", already.ID: "suspended"}
	for _, c := range m.Clients("s1") {
		if c.Status != want[c.ID] {
			t.Errorf("client %s: status = %q, want %q", c.Name, c.Status, want[c.ID])
		}
	}
}
