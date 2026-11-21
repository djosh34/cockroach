#! /usr/bin/env expect -f

source [file join [file dirname $argv0] common.tcl]

set certs_dir "/certs"
set ::env(COCKROACH_INSECURE) "false"
set ::env(COCKROACH_HOST) "localhost"

# The node and client certs come from $certs_dir, signed by one CA.
set config [write_config $certs_dir]
set testuser_url "postgresql://testuser@localhost:26257/?sslmode=verify-full&sslrootcert=$certs_dir/ca.crt&sslcert=$certs_dir/client.testuser.crt&sslkey=$certs_dir/client.testuser.key"
# root's client cert, for the HTTP API.
set curl "curl -s --cacert $certs_dir/ca.crt --cert $certs_dir/client.root.crt --key $certs_dir/client.root.key"

spawn /bin/bash
send "PS1=':''/# '\r"

set prompt ":/# "
eexpect $prompt

start_test "Check that --insecure reports that the server is really insecure"
send "$argv start-single-node --host=localhost --insecure\r"
eexpect "WARNING: ALL SECURITY CONTROLS HAVE BEEN DISABLED"
eexpect "node starting"
interrupt
eexpect $prompt
end_test

start_test "Check that a secure node needs --config"
send "$argv start-single-node --host=localhost\r"
eexpect "a secure node needs --config"
eexpect $prompt
end_test

proc start_secure_server {argv config extra} {
    report "BEGIN START SECURE SERVER"
    # We add some arbitrary env vars here to check env var
    # exposure in one of the sub tests below.
    system "env AWS_ACCESS_KEY=supersecret TERM=mycoolterm GODEBUG=mycooldebugvar $argv start-single-node --host=localhost --socket-dir=. --config=$config --pid-file=server_pid -s=path=logs/db --background $extra >>expect-cmd.log 2>&1;
            $argv sql --config=$config -e 'select 1'"
    report "END START SECURE SERVER"
}

start_secure_server $argv $config ""

start_test "Check that the env vars are properly reported in the log file."
# NB: this grep checks the aws access key was redacted.
system "grep AWS_ACCESS_KEY='\\.\\.\\.' logs/db/logs/cockroach.log"
system "grep GODEBUG=mycooldebugvar logs/db/logs/cockroach.log"
# NB: this grep checks the value of the TERM var was enclosed
# in redaction markers.
system "grep TERM=..*mycoolterm logs/db/logs/cockroach.log"
end_test

start_test "Check 'node ls' works with --config."
send "$argv node ls --config=$config\r"
eexpect "id"
eexpect "1"
eexpect "1 row"
eexpect $prompt
end_test

start_test "Can create users."
send "$argv sql -e 'create user testuser' --config=$config\r"
eexpect $prompt
end_test

start_test "A user logs in with its client cert"
send "$argv sql --no-line-editor --url='$testuser_url'\r"
eexpect "testuser@"
send "\\q\r"
eexpect $prompt
end_test

start_test "A client cert does not log in as another user"
send "$argv sql --no-line-editor --url='$testuser_url' --user=root -e 'select 1'\r"
eexpect "no client rule maps certificate"
eexpect $prompt
end_test

start_test "Check that root's client cert works on the HTTP API."
send "$curl https://localhost:8080/_admin/v1/settings\r"
eexpect "cluster.organization"
eexpect $prompt
end_test

start_test "Check that env vars are reported in the node report"
send "$curl https://localhost:8080/_status/nodes/1 > logs/db/node.txt\r"
eexpect $prompt
system "grep AWS_ACCESS_KEY='\\.\\.\\.' logs/db/node.txt"
system "grep GODEBUG=mycooldebugvar logs/db/node.txt"
system "grep TERM=mycoolterm logs/db/node.txt"
end_test

start_test "Check that env vars are NOT reported in the diagnostic report"
send "$curl https://localhost:8080/_status/diagnostics/1 > logs/db/diag.txt\r"
eexpect $prompt
system "if grep AWS_ACCESS_KEY logs/db/diag.txt; then echo FAIL; fi"
system "if grep TERM logs/db/diag.txt; then echo FAIL; fi"
system "if grep GODEBUG logs/db/diag.txt; then echo FAIL; fi"
end_test

send "exit 0\r"
eexpect eof

stop_server $argv
