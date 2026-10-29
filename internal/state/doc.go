// Package state persists VM ownership and operation progress and supplies host locks.
// Lima stores backend state separately; guest home contents live under configuration home.root.
//
//	<state root>/
//	├─ instances/<name>/
//	│  ├─ identity.json          immutable ownership
//	│  ├─ instance.json          status, generation ID, recovery message
//	│  └─ generations/<id>/      inputs prepared by internal/vm
//	├─ modules/<name>/           imported trees managed by internal/modules
//	├─ homes/<name>-<id>.json    ownership retained after VM deletion
//	├─ locks/instances/<name>.lock
//	├─ locks/registry.lock
//	└─ runtime/guestagents/      archives materialized by internal/bundle
//
// [NewStore] resolves the root without creating it.
// Reads and removal recheck the root and refuse a symlink introduced after construction.
// [Store.Save] rejects changes to an existing identity.
// identity.json and instance.json are replaced atomically as separate files, not as one transaction.
// Ownership can be read even when the runtime record is damaged.
// Records require UTF-8 JSON with every expected field present exactly once.
//
// [Store.Remove] removes VM records and generation inputs, never home contents.
// [Store.PreserveHome] archives ownership metadata, not a copy of the home.
// [Store.ForgetHome] removes only the matching archived identity.
package state
