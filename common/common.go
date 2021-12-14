package common

import (
	"crypto/tls"
	"crypto/x509"
	b64 "encoding/base64"
	"io/ioutil"
	"log"

	"github.com/google/uuid"
	"google.golang.org/grpc/credentials"
)

// S2B parse b64 to bytes
// ToDo: error handling
func S2B(s string) []byte {
	decoded, _ := b64.StdEncoding.DecodeString(s)
	return decoded
}

// B2S encode bytes to string
func B2S(bytes []byte) string {
	return b64.StdEncoding.EncodeToString(bytes)
}

// RandomUUID shortcut
// ToDo: error handling
func RandomUUID() (ruid []byte) {
	uuidv4, err := uuid.NewRandom()
	if err != nil {
		log.Fatalf("Could not generate UUID: %v", err)
	}

	ruid, err = uuidv4.MarshalBinary()
	if err != nil {
		log.Fatalf("Could not generate UUID: %v", err)
	}

	return
}

// UUID2bytes parse uuid to bytes
// ToDo: error handling
func UUID2bytes(uuidString string) []byte {
	u, _ := uuid.Parse(uuidString)
	b, _ := u.MarshalBinary()
	return b
}

// Bytes2uuid marshal bytes to string
// ToDo: error handling
func Bytes2uuid(b []byte) string {
	u, err := uuid.FromBytes(b)
	if err != nil {
		log.Println(err)
	}

	s, _ := u.MarshalText()
	return string(s)
}

//LoadTLSKeys Parse TLS keys for client
func LoadTLSKeys(clientCertPath, clientKeyPath, caCertPath *string) credentials.TransportCredentials {
	peerCert, err := tls.LoadX509KeyPair(*clientCertPath, *clientKeyPath)
	if err != nil {
		log.Fatalf("load peer cert/key error:%v", err)
	}
	caCert, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		log.Fatalf("read ca cert file error:%v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	ta := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{peerCert},
		RootCAs:      caCertPool,
	})
	return ta
}
