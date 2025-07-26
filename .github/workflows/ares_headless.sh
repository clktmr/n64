#!/bin/sh
xvfb-run ares \
	--setting Input/Driver=None \
	--setting Video/Driver=None \
	--setting Audio/Driver=None \
	"$@"
