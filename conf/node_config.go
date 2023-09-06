// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package conf

import (
	"os"
	"path/filepath"
)

const (
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

type NodeConfig struct {
	NodePrivate string `json:"private" yaml:"private"`
	HTTP        bool   `json:"http" yaml:"http" `
	HTTPHost    string `json:"http_host" yaml:"http_host" `
	HTTPPort    string `json:"http_port" yaml:"http_port"`
	HTTPApi     string `json:"http_api" yaml:"http_api"`
	WS          bool   `json:"ws" yaml:"ws" `
	WSHost      string `json:"ws_host" yaml:"ws_host" `
	WSPort      string `json:"ws_port" yaml:"ws_port"`
	WSApi       string `json:"ws_api" yaml:"ws_api"`
	IPCPath     string `json:"ipc_path" yaml:"ipc_path"`
	DataDir     string `json:"data_dir" yaml:"data_dir"`
	Miner       bool   `json:"miner" yaml:"miner"`

	AuthRPC bool `json:"auth_rpc" yaml:"auth_rpc"`
	// AuthAddr is the listening address on which authenticated APIs are provided.
	AuthAddr string `json:"auth_addr" yaml:"auth_addr"`

	// AuthPort is the port number on which authenticated APIs are provided.
	AuthPort int `json:"auth_port" yaml:"auth_port"`

	// AuthVirtualHosts is the list of virtual hostnames which are allowed on incoming requests
	// for the authenticated api. This is by default {'localhost'}.
	AuthVirtualHosts []string `json:"auth_virtual_hosts" yaml:"auth_virtual_hosts"`

	// JWTSecret is the path to the hex-encoded jwt secret.
	JWTSecret string `json:"jwt_secret" yaml:"jwt_secret"`

	// KeyStoreDir is the file system folder that contains private keys. The directory can
	// be specified as a relative path, in which case it is resolved relative to the
	// current directory.
	//
	// If KeyStoreDir is empty, the default location is the "keystore" subdirectory of
	// DataDir. If DataDir is unspecified and KeyStoreDir is empty, an ephemeral directory
	// is created by New and destroyed when the node is stopped.
	KeyStoreDir string `json:"key_store_dir" yaml:"key_store_dir"`

	// ExternalSigner specifies an external URI for a clef-type signer
	ExternalSigner string `json:"external_signer" yaml:"external_signer"`

	// UseLightweightKDF lowers the memory and CPU requirements of the key store
	// scrypt KDF at the expense of security.
	UseLightweightKDF bool `json:"use_lightweight_kdf" yaml:"use_lightweight_kdf"`

	// InsecureUnlockAllowed allows user to unlock accounts in unsafe http environment.
	InsecureUnlockAllowed bool `json:"insecure_unlock_allowed" yaml:"insecure_unlock_allowed"`

	PasswordFile string `json:"password_file" yaml:"password_file"`
}

// KeyDirConfig determines the settings for keydirectory
func (c *NodeConfig) KeyDirConfig() (string, error) {
	var (
		keydir string
		err    error
	)
	switch {
	case filepath.IsAbs(c.KeyStoreDir):
		keydir = c.KeyStoreDir
	case c.DataDir != "":
		if c.KeyStoreDir == "" {
			keydir = filepath.Join(c.DataDir, datadirDefaultKeyStore)
		} else {
			keydir, err = filepath.Abs(c.KeyStoreDir)
		}
	case c.KeyStoreDir != "":
		keydir, err = filepath.Abs(c.KeyStoreDir)
	}
	return keydir, err
}

// getKeyStoreDir retrieves the key directory and will create
// and ephemeral one if necessary.
func getKeyStoreDir(conf *NodeConfig) (string, bool, error) {
	keydir, err := conf.KeyDirConfig()
	if err != nil {
		return "", false, err
	}
	isEphemeral := false
	if keydir == "" {
		// There is no datadir.
		keydir, err = os.MkdirTemp("", "amazechain-keystore")
		isEphemeral = true
	}

	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(keydir, 0700); err != nil {
		return "", false, err
	}

	return keydir, isEphemeral, nil
}

// ExtRPCEnabled returns the indicator whether node enables the external
// RPC(http, ws or graphql).
func (c *NodeConfig) ExtRPCEnabled() bool {
	return c.HTTPHost != "" || c.WSHost != ""
}
