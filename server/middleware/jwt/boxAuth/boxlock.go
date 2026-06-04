// Package boxauth provides box-level locking — preventing concurrent
// modifications to the same box. The implementation is currently a stub;
// add a Redis-backed distributed lock or a database advisory lock here when
// you need to protect multi-step box operations from race conditions.
package boxauth

func Lock(boxName string, userId string) error {
	// TODO: acquire a distributed lock on this box for this user
	return nil
}

func Unlock(boxName string, userId string) error {
	// TODO: release the lock acquired in Lock
	return nil
}

func IsLocked(boxName string, userId string) (bool, error) {
	// TODO: return true if another operation currently holds the lock
	return false, nil
}
