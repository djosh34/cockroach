#! /usr/bin/env expect -f

source [file join [file dirname $argv0] common.tcl]

start_server $argv

spawn $argv sql --no-line-editor

start_test "Check that the client starts with the welcome message."
eexpect "# Welcome to the patchroaddb SQL shell."
end_test

start_test "Check that the client reports the server version, and correctly detects the version is the same as the client."
eexpect "# Server version: patchroaddb"
eexpect "(same version as client)"
end_test

start_test "Check that cluster ID is reported as well."
eexpect "# Cluster ID: "
end_test

start_test "Check that the help part of the introductory message is at the end."
eexpect "for a brief introduction"
end_test

eexpect root@

start_test "Check that a reconnect without version change is quiet."
# We need to force since the open connection may prevent a quick
# graceful shutdown.
force_stop_server $argv
start_server $argv
send "select 1;\r"
eexpect "connection closed unexpectedly"
# Check that the prompt immediately succeeds the error message
eexpect "connection lost"
eexpect "opening new connection: all session settings will be lost"
eexpect "root@"

# Check the reconnect did succeed - this also resets the connection state to good.
send "select 1;\r"
eexpect "1 row"
eexpect root@
end_test

start_test "Check that the client picks up a new cluster ID."
force_stop_server $argv
system "mv logs/db logs/db-bak"
start_server $argv
send "select 1;\r"
eexpect "opening new connection"
eexpect "the cluster ID has changed!"
eexpect "Previous ID:"
eexpect "New Cluster ID:"
eexpect root@
end_test

send_eof
eexpect eof

stop_server $argv

start_test "Check that a release binary ignores the testing version override."
# Only crdb_test builds read COCKROACH_TESTING_VERSION_OVERRIDE, so client and
# server keep the same version.
set env(COCKROACH_TESTING_VERSION_OVERRIDE) "v0.1.0-fakever"
start_server $argv
spawn $argv sql --no-line-editor
send "select 1;\r"
eexpect "(same version as client)"
eexpect root@
end_test
unset env(COCKROACH_TESTING_VERSION_OVERRIDE)

send_eof
eexpect eof

stop_server $argv
