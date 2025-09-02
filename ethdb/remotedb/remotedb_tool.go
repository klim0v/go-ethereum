// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package remotedb implements the key-value database layer based on a remote geth
// node. Under the hood, it utilises the `debug_dbGet` method to implement a
// read-only database.
// There really are no guarantees in this database, since the local geth does not
// exclusive access, but it can be used for basic diagnostics of a remote node.
package remotedb

func (db *Database) SetupBlockHistory(blockHistory uint64) error {
	panic("not supported")
}
