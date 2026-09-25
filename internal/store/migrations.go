// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package store

import "net/netip"

// This file holds the one-shot migrations that bring a config written by an
// older release - web_config.json and the .conf files it points at - up to
// what the current code expects. They run at startup, from manager.New,
// before anything reads the config for real.
//
// To add one:
//
//   - bump SchemaVersion to the revision the migration produces and describe
//     that revision in the list on it;
//   - give the migration its own constant holding that same revision, open
//     its body with "if cfg.SchemaVersion >= <that constant> { return }" so a
//     later bump cannot re-run it, and call it from Migrate.

// SchemaVersion is the current web_config.json schema revision.
//
//	3 - the shape the first release writes: every client lives in the Clients
//	    list of its server, and every peer marker in a server .conf carries
//	    the client ID.
//	4 - every client's ServerID actually names the server it lives in; see
//	    repairClientServerIDs.
//	5 - a legacy auto-detected IPv4 is no longer stored as an explicit endpoint
//	    override, allowing later public-IP refreshes to affect client exports.
//
// It starts at 3 rather than 1 because pre-release builds already stamped 1
// and 2 into deployed configs. Numbering the first release below what is
// already on disk would make those files look newer than the code, and the
// next migration - guarded by "the config is older than this revision" -
// would never run on them.
const SchemaVersion = 5

// Migrate applies every migration cfg still needs and stamps the result, so
// none of them run twice. It reports whether cfg changed and therefore needs
// saving; a config that is already current is left untouched.
func Migrate(cfg *AppConfig) (changed bool) {
	if cfg.SchemaVersion >= SchemaVersion {
		return false
	}

	// Migrations go here, oldest first.
	repairClientServerIDs(cfg)
	clearLegacyAutoEndpoints(cfg)

	cfg.SchemaVersion = SchemaVersion
	return true
}

const schemaEndpointOverrides = 5

// clearLegacyAutoEndpoints repairs the old create behavior that copied an
// automatically detected IPv4 (or its unknown placeholder) into both fields.
// Equal DNS names are deliberately preserved: they were explicit operator
// choices, while automatic detection has only ever returned IPv4 addresses.
func clearLegacyAutoEndpoints(cfg *AppConfig) {
	if cfg.SchemaVersion >= schemaEndpointOverrides {
		return
	}
	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		if srv.Endpoint == "" || srv.Endpoint != srv.PublicIP {
			continue
		}
		addr, err := netip.ParseAddr(srv.Endpoint)
		if (err == nil && addr.Is4()) || srv.Endpoint == "YOUR_SERVER_IP" {
			srv.Endpoint = ""
		}
	}
}

const schemaClientServerIDs = 4

// repairClientServerIDs rewrites every client's ServerID from the server it is
// stored under.
//
// Releases up to 3.1.4 took that id straight from the route parameter and kept
// it, but fiber hands route parameters back as strings pointing into the
// request buffer it recycles the moment the handler returns. The id therefore
// decayed into a slice of some later request ("tatus5" off a /api/system/status
// path, "i-sett" off an .../i-settings one) and any save after that wrote the
// garbage to disk. Membership in the server's Clients list is what actually
// binds a client to its server, so the correct value is always to hand and
// nothing else needs repairing.
func repairClientServerIDs(cfg *AppConfig) {
	if cfg.SchemaVersion >= schemaClientServerIDs {
		return
	}

	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		for j := range srv.Clients {
			if srv.Clients[j].ServerID != srv.ID {
				srv.Clients[j].ServerID = srv.ID
			}
		}
	}
}
