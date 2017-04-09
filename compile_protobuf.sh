#!/bin/bash

SRCROOT=resources/protobuf-definitions
TGTROOT=packet

protoc -I=$SRCROOT --go_out=$TGTROOT $SRCROOT/*.proto
