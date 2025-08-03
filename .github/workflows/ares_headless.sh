#!/bin/sh
xvfb-run -s "-terminate" ares \
	--setting Input/Driver=None \
	--setting Video/Driver=None \
	--setting Audio/Driver=None \
	"$@"
