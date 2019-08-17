package sweetdb

import (
	"bytes"
	"encoding/json"
	"github.com/go-errors/errors"
	"go.etcd.io/bbolt"
)

func (db *DB) setJSON(bucket []byte, bucketKey []byte, v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}

		if err := bucket.Put(bucketKey, payload); err != nil {
			return err
		}

		return nil
	})
}

func (db *DB) getJSON(bucket []byte, bucketKey []byte, v interface{}) error {
	var lightningNode = &LightningNode{}

	err := db.View(func(tx *bbolt.Tx) error {
		// First fetch the bucket
		bucket := tx.Bucket(settingsBucket)
		if bucket == nil {
			return nil
		}

		lightningNodeBytes := bucket.Get(lightningNodeKey)
		if lightningNodeBytes == nil || bytes.Equal(lightningNodeBytes, []byte("null")) {
			lightningNode = nil
			return nil
		}

		err := json.Unmarshal(lightningNodeBytes, &v)
		if err != nil {
			return errors.Errorf("Could not unmarshal data: %v", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
