#!/usr/bin/env bash

NAME=chirpstack-fuota-server
DAEMON_USER=fuotaserver
DAEMON_GROUP=fuotaserver

# create user
id $DAEMON_USER &>/dev/null
if [[ $? -ne 0 ]]; then
	useradd --system -U -M $DAEMON_USER -s /bin/false -d /etc/$NAME
fi

# set the configuration owner / permissions
if [[ -f /etc/$NAME/$NAME.toml ]]; then
	chown -R $DAEMON_USER:$DAEMON_GROUP /etc/$NAME
	chmod 750 /etc/$NAME
	chmod 640 /etc/$NAME/$NAME.toml
fi

# show message on install
if [[ $? -eq 0 ]]; then
	echo -e "\n\n\n"
	echo "---------------------------------------------------------------------------------"
	echo "The configuration file is located at:"
	echo " /etc/$NAME/$NAME.toml"
	echo ""
	echo "Some helpful commands for $NAME:"
	echo ""
	echo "Start:"
	echo " $ sudo systemctl start $NAME"
	echo ""
	echo "Restart:"
	echo " $ sudo systemctl restart $NAME"
	echo ""
	echo "Stop:"
	echo " $ sudo systemctl stop $NAME"
	echo ""
	echo "Display logs:"
	echo " $ sudo journalctl -f -n 100 -u $NAME"
	echo "---------------------------------------------------------------------------------"
	echo -e "\n\n\n"
fi

# install systemd
systemctl daemon-reload
systemctl enable $NAME

# restart on upgrade
if [[ -n $2 ]]; then
	systemctl daemon-reload
	systemctl restart $NAME
fi
