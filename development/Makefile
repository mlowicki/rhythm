# https://superuser.com/a/505209
blockzk:
	(sudo pfctl -sr 2>/dev/null; echo "block drop quick on lo0 proto tcp from any to any port = 2181") | sudo pfctl -f - 2>/dev/null
unblockzk:
	sudo pfctl -f /etc/pf.conf 
