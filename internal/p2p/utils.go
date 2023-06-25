package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/amazechain/amc/api/protocol/msg_proto"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/p2p/enr"
	"github.com/amazechain/amc/utils"
	"net"
	"os"
	"path"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
)

const keyPath = "network-keys"
const metaDataPath = "metaData"

const dialTimeout = 1 * time.Second

// SerializeENR takes the enr record in its key-value form and serializes it.
func SerializeENR(record *enr.Record) (string, error) {
	if record == nil {
		return "", errors.New("could not serialize nil record")
	}
	buf := bytes.NewBuffer([]byte{})
	if err := record.EncodeRLP(buf); err != nil {
		return "", errors.Wrap(err, "could not encode ENR record to bytes")
	}
	enrString := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	return enrString, nil
}

// Determines a private key for p2p networking from the p2p service's
// configuration struct. If no key is found, it generates a new one.
func privKey(cfg *conf.P2PConfig) (*ecdsa.PrivateKey, error) {
	defaultKeyPath := path.Join(cfg.DataDir, keyPath)
	privateKeyPath := cfg.PrivateKey

	// PrivateKey cli flag takes highest precedence.
	if privateKeyPath != "" {
		return privKeyFromFile(cfg.PrivateKey)
	}

	_, err := os.Stat(defaultKeyPath)
	defaultKeysExist := !os.IsNotExist(err)
	if err != nil && defaultKeysExist {
		return nil, err
	}
	// Default keys have the next highest precedence, if they exist.
	if defaultKeysExist {
		return privKeyFromFile(defaultKeyPath)
	}
	// There are no keys on the filesystem, so we need to generate one.
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	// If the StaticPeerID flag is set, save the generated key as the default
	// key, so that it will be used by default on the next node start.
	if cfg.StaticPeerID {
		rawbytes, err := priv.Raw()
		if err != nil {
			return nil, err
		}
		dst := make([]byte, hex.EncodedLen(len(rawbytes)))
		hex.Encode(dst, rawbytes)
		if err := os.WriteFile(defaultKeyPath, dst, 0600); err != nil {
			return nil, err
		}
		log.Info("Wrote network key to file")
		// Read the key from the defaultKeyPath file just written
		// for the strongest guarantee that the next start will be the same as this one.
		return privKeyFromFile(defaultKeyPath)
	}
	return utils.ConvertFromInterfacePrivKey(priv)
}

// Retrieves a p2p networking private key from a file path.
func privKeyFromFile(path string) (*ecdsa.PrivateKey, error) {
	src, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		log.Error("Error reading private key from file", "err", err)
		return nil, err
	}
	dst := make([]byte, hex.DecodedLen(len(src)))
	_, err = hex.Decode(dst, src)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode hex string")
	}
	unmarshalledKey, err := crypto.UnmarshalSecp256k1PrivateKey(dst)
	if err != nil {
		return nil, err
	}
	return utils.ConvertFromInterfacePrivKey(unmarshalledKey)
}

// Retrieves node p2p metadata from a set of configuration values
// from the p2p service.
// TODO: Figure out how to do a v1/v2 check.
func metaDataFromConfig(cfg *conf.P2PConfig) (*msg_proto.Metadata, error) {
	return &msg_proto.Metadata{}, nil
}

// Attempt to dial an address to verify its connectivity
func verifyConnectivity(addr string, port int, protocol string) {
	if addr != "" {
		a := net.JoinHostPort(addr, fmt.Sprintf("%d", port))

		conn, err := net.DialTimeout(protocol, a, dialTimeout)
		if err != nil {
			log.Warn("IP address is not accessible", "err", err, "protocol", protocol, "address", a)
			return
		}
		if err := conn.Close(); err != nil {
			log.Debug("Could not close connection", "err", err)
		}
	}
}
