package main

import (
	"fmt"
	"log"
	"os"

	pb "github.com/ice2heart/proxyu_client/serialize"
	"google.golang.org/protobuf/proto"

	bolt "go.etcd.io/bbolt"
)

var (
	db *bolt.DB
)

// WriteUserData if you successfully got it
func WriteUserData(subject *[32]byte, data *[16]byte, mime *string, payload []byte) error {
	log.Printf("Write user data %v", db.Stats())
	err := db.Update(func(tx *bolt.Tx) error {
		mb, err := tx.CreateBucketIfNotExists([]byte("Data"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		b, err := mb.CreateBucketIfNotExists(subject[:])
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		m := &pb.UserData{
			Mime:  *mime,
			Value: payload,
		}
		mBytes, err := proto.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal error: %s", err)
		}
		err = b.Put(data[:], mBytes)
		return err
	})
	if err != nil {
		// fmt.Fprintf(os.Stderr, "WriteUserData error: %v", err)
		return err
	}
	return nil
}

type UserData struct {
	Data  [16]byte
	Mime  string
	Value []byte
}

// GetAllUserData extract all data for user
func GetAllUserData(subject *[32]byte) (ret []UserData, err error) {
	err = db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte("Data"))
		if b == nil {
			return nil
		}
		ub := b.Bucket(subject[:])
		if ub == nil {
			return nil
		}
		ub.ForEach(func(k, v []byte) error {

			var item UserData
			copy(item.Data[:], k)
			userData := &pb.UserData{}
			proto.Unmarshal(v, userData)
			item.Mime = userData.Mime
			// fmt.Printf("key=%v, value=%v mime=%s\n", k, userData.Value, userData.Mime)
			// copy(item.Value, userData.Value)
			item.Value = userData.Value
			ret = append(ret, item)
			return nil
		})
		return nil
	})
	return
}

// ExtractUserData userdata + mimetype of data
func ExtractUserData(subject *[32]byte, data *[16]byte) (payload []byte, mime string) {
	db.View(func(tx *bolt.Tx) error {
		pbd := tx.Bucket([]byte("Data"))
		if pbd == nil {
			return nil
		}
		sb := pbd.Bucket(subject[:])
		if sb == nil {
			return nil
		}
		v := sb.Get(data[:])
		if v != nil {
			userData := &pb.UserData{}
			proto.Unmarshal(v, userData)

			payload = make([]byte, len(userData.GetValue()))
			copy(payload, userData.GetValue())
			mime = userData.GetMime()
		}
		return nil
	})
	return
}

//WriteSession for user
func WriteSession(id *[16]byte, pubkey *[32]byte) error {
	err := db.Update(func(tx *bolt.Tx) error {
		mb, err := tx.CreateBucketIfNotExists([]byte("Session"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		m := &pb.UserInfo{
			Uuid:   id[:],
			Pubkey: pubkey[:],
		}
		mBytes, err := proto.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal error: %s", err)
		}
		err = mb.Put(id[:], mBytes)
		return err
	})
	if err != nil {
		// fmt.Fprintf(os.Stderr, "WriteUserData error: %v", err)
		return err
	}
	return nil
}

//GetSession for user
func GetSession(id *[16]byte) (pubkey []byte) {
	db.View(func(tx *bolt.Tx) error {
		pbd := tx.Bucket([]byte("Session"))
		if pbd == nil {
			return nil
		}
		v := pbd.Get(id[:])
		if v == nil {
			return nil
		}
		userSession := &pb.UserInfo{}
		proto.Unmarshal(v, userSession)

		pubkey = make([]byte, len(userSession.GetPubkey()))
		copy(pubkey, userSession.GetPubkey())
		return nil
	})
	return
}

//OpenDB init db process
func OpenDB(fileName *string, intSig chan os.Signal) {
	var err error
	db, err = bolt.Open(*fileName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("DB is open")
	defer db.Close()
	<-intSig
}
