package main

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod/rcpmodtest"
)

func TestContract(t *testing.T) { rcpmodtest.Module(t, module) }
