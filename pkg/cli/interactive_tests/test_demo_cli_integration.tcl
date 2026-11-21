#! /usr/bin/env expect -f

source [file join [file dirname $argv0] common.tcl]

# A demo cluster is always insecure.
set ::env(COCKROACH_INSECURE) "true"

spawn $argv demo --empty --no-line-editor --multitenant=true --log-dir=logs
eexpect "Welcome"

start_test "Check that the cli connect instructions get printed out."
eexpect "defaultdb>"
send "\\demo ls\r"
eexpect "Application tenant"
eexpect "(cli)"
eexpect "patchroaddb sql --insecure"
end_test

eexpect "defaultdb>"

set demo_spawn_id $spawn_id

start_test "Check that a SQL shell can connect to the app tenant without special arguments"
spawn $argv sql --no-line-editor -u root
eexpect "Welcome"
expect {
    "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT" { exit 1 }
    root@ {}
}
eexpect "defaultdb>"
send "table system.tenants;\r"
eexpect "ERROR"
eexpect "does not exist"
eexpect "defaultdb>"
send "\\q\r"
eexpect eof

spawn $argv sql --no-line-editor -u root -d cluster:demoapp
eexpect "Welcome"
expect {
    "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT" { exit 1 }
    root@ {}
}
eexpect "demoapp/defaultdb>"
send "\\q\r"
eexpect eof
end_test

start_test "Check that a SQL shell can connect to the system tenant without special arguments"
spawn $argv sql --no-line-editor -u root -d cluster:system
eexpect "Welcome"
eexpect "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT"
eexpect root@
eexpect "system/defaultdb>"
send "table system.tenants;\r"
eexpect "2 rows"
eexpect "system/defaultdb>"
send "\\q\r"
eexpect eof
end_test

# Switch between tenants after the cluster option has been set in a URL.
spawn $argv sql --no-line-editor --url "postgresql://root@localhost:26257?options=-ccluster=demoapp&sslmode=disable"
eexpect "Welcome"
eexpect root@
eexpect "demoapp/defaultdb>"
send "use postgres;\r"
eexpect "demoapp/postgres>"

start_test "Check that the session can be switched to the system tenant with the connect command"
send "\\connect cluster:system\r"
eexpect root@
eexpect "system/defaultdb>"

send "\\connect cluster:demoapp\r"
eexpect root@
eexpect "demoapp/defaultdb>"

# Also check the DB name can still be specified.
send "\\connect cluster:system/postgres\r"
eexpect root@
eexpect "system/postgres>"
end_test

send "\\q\r"
eexpect eof

start_test "Check that clients can use the unix socket to connect to the app tenant."
set sockdir $env(HOME)/.cockroach-demo
spawn $argv sql --no-line-editor --url "postgresql://root@/defaultdb?host=$sockdir&port=26257"
eexpect "Welcome"
expect {
    "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT" { exit 1 }
    root@ {}
}
eexpect "defaultdb>"
send "table system.tenants;\r"
eexpect "ERROR"
eexpect "does not exist"
eexpect "defaultdb>"
send "\\q\r"
eexpect eof
end_test

start_test "Check that clients can use the unix socket to connect to the system tenant."
spawn $argv sql --no-line-editor --url "postgresql://root@/defaultdb?host=$sockdir&options=-ccluster%3Dsystem&port=26257"
eexpect "Welcome"
eexpect "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT"
eexpect root@
eexpect "system/defaultdb>"
send "table system.tenants;\r"
eexpect "2 rows"
eexpect "system/defaultdb>"
send "\\q\r"
eexpect eof
end_test

set spawn_id $demo_spawn_id
send "\\q\r"
eexpect eof

start_test "Check that the warning for the system tenant is not printed when there are no secondary tenants"
spawn $argv demo --empty --no-line-editor --multitenant=false --log-dir=logs
eexpect "Welcome"
expect {
    "ATTENTION: YOU ARE CONNECTED TO THE SYSTEM TENANT" { exit 1 }
    root@ {}
}
eexpect "defaultdb>"
send "\\q\r"
eexpect eof
end_test

# Regression test for #95135.
start_test "Verify that the demo command did not leave tenant directories around."
system "if ls tenant-* >logs/tenants 2>&1; then echo Strays; cat logs/tenants; exit 1; fi"
end_test
