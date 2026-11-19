#!/usr/bin/env bash
# Regenerates all generated code in the repository.
#
# Usage: build/generate.sh [step...]
#
# With no arguments it runs every step, in this order: proto, parser, optgen,
# execgen, stringer, mocks, misc, schemachanger, logictest, docs.
#
# The Go tools come from go.mod (the tool directives, or packages of this
# repository). protoc is downloaded once into bin/ and checked against a
# pinned checksum.
set -euo pipefail

cd "$(dirname "$0")/.."
root=$(pwd)

PROTOC_VERSION=3.9.2
declare -A PROTOC_SHA256=(
  [x86_64]=0d9034a3b02bd77edf5ef926fb514819a0007f84252c5e6a6391ddfc4189b904
  [aarch_64]=55dba3d64d45fc79e4c93d376b2e578dfec913eaa2b6f071beb2460956a17db6
)

bin="$root/bin/generate"
mkdir -p "$bin"
export PATH="$bin:$PATH"
export COCKROACH_INTERNAL_DISABLE_METAMORPHIC_TESTING=true

# build_tool builds a Go tool from go.mod or this repository into $bin.
build_tool() {
  local name=$1 pkg=$2
  if [[ $pkg == ./* ]]; then
    go build -o "$bin/$name" "$pkg"
  else
    go build -o "$bin/$name" "$(go list -f '{{.ImportPath}}' "$pkg")"
  fi
}

module_dir() {
  go list -m -f '{{.Dir}}' "$1"
}

install_protoc() {
  local arch
  case "$(uname -m)" in
    x86_64) arch=x86_64 ;;
    aarch64 | arm64) arch=aarch_64 ;;
    *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
  esac
  local dir="$bin/protoc-$PROTOC_VERSION"
  if [[ ! -x $dir/bin/protoc ]]; then
    local zip="$bin/protoc-$PROTOC_VERSION.zip"
    curl -sSfL -o "$zip" \
      "https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-$arch.zip"
    echo "${PROTOC_SHA256[$arch]}  $zip" | sha256sum -c --quiet
    rm -rf "$dir"
    unzip -q -d "$dir" "$zip"
    rm "$zip"
  fi
  PROTOC="$dir/bin/protoc"
  PROTOC_INCLUDE="$dir/include"
}

# proto_include links the .proto files the protos import from other modules
# into $bin/proto-include, under the paths the imports use.
proto_include() {
  # The protos import these files from other modules under these paths.
  local inc="$bin/proto-include"
  rm -rf "$inc"
  mkdir -p "$inc/raft" "$inc/google" "$inc/io/prometheus"
  ln -s "$(module_dir github.com/gogo/protobuf)/gogoproto" "$inc/gogoproto"
  ln -s "$(module_dir github.com/cockroachdb/errors)/errorspb" "$inc/errorspb"
  ln -s "$(module_dir go.etcd.io/raft/v3)" "$inc/raft/v3"
  ln -s "$(module_dir github.com/gogo/googleapis)/google/api" "$inc/google/api"
  ln -s "$(module_dir github.com/prometheus/client_model)/io/prometheus/client" "$inc/io/prometheus/client"

}

gen_proto() {
  install_protoc
  build_tool protoc-gen-gogoroach ./pkg/cmd/protoc-gen-gogoroach
  build_tool protoc-gen-grpc-gateway github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway

  proto_include

  local m=(
    Mraft/v3/raftpb/raft.proto=go.etcd.io/raft/v3/raftpb
    Merrorspb/errors.proto=github.com/cockroachdb/errors/errorspb
    Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations
    Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types
    Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types
    Mgoogle/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor
    Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types
  )
  # Map every proto of this repository to its Go package, since some of them
  # do not set go_package to the full import path.
  local f
  for f in $(git ls-files 'pkg/*.proto' | sed 's|^pkg/||'); do
    m+=("M$f=github.com/cockroachdb/cockroach/pkg/$(dirname "$f")")
  done
  local opts
  opts="plugins=grpc,$(IFS=,; echo "${m[*]}"),paths=source_relative"

  # Each directory with .proto files is one Go package.
  local dir
  for dir in $(git ls-files 'pkg/*.proto' | xargs -n1 dirname | sort -u); do
    rm -f "$dir"/*.pb.go "$dir"/*.pb.gw.go
    local files
    files=$(git ls-files "$dir/*.proto" | grep -v "^$dir/.*/" | sed 's|^pkg/||')
    # shellcheck disable=SC2086
    "$PROTOC" -Ipkg -I"$bin/proto-include" -I"$PROTOC_INCLUDE" \
      --plugin=protoc-gen-gogoroach="$bin/protoc-gen-gogoroach" \
      --gogoroach_out="$opts:pkg" $files
    if grep -q 'google.api.http' $(sed 's|^|pkg/|' <<<"$files"); then
      # shellcheck disable=SC2086
      "$PROTOC" -Ipkg -I"$bin/proto-include" -I"$PROTOC_INCLUDE" \
        --plugin=protoc-gen-grpc-gateway="$bin/protoc-gen-grpc-gateway" \
        --grpc-gateway_out="logtostderr=true,request_context=true,paths=source_relative:pkg" $files
    fi
    gofmt -w "$dir"/*.pb*.go
  done
}

gen_parser() {
  build_tool goyacc golang.org/x/tools/cmd/goyacc
  local tmp
  tmp=$(mktemp -d)
  (cd "$tmp" && bash "$root/pkg/sql/lexbase/sql-gen.sh" "$root/pkg/sql/parser/sql.y" sql \
    "$root/pkg/sql/parser/replace_help_rules.awk" "$root/pkg/sql/parser/sql.go" "$bin/goyacc")
  (cd "$tmp" && bash "$root/pkg/sql/lexbase/sql-gen.sh" "$root/pkg/sql/pgrepl/pgreplparser/pgrepl.y" pgrepl \
    "" "$root/pkg/sql/pgrepl/pgreplparser/pgrepl.go" "$bin/goyacc")
  rm -rf "$tmp"

  bash pkg/sql/parser/help_gen_test.sh <pkg/sql/parser/sql.go >pkg/sql/parser/helpmap_test.go
  awk -f pkg/sql/parser/help.awk <pkg/sql/parser/sql.y >pkg/sql/parser/help_messages.go
  go run ./pkg/sql/lexbase/allkeywords <pkg/sql/parser/sql.y >pkg/sql/lexbase/keywords.go
  awk -f pkg/sql/parser/reserved_keywords.awk <pkg/sql/parser/sql.y >pkg/sql/lexbase/reserved_keywords.go
  {
    echo "// Code generated by make. DO NOT EDIT."
    echo "// GENERATED FILE DO NOT EDIT"
    echo
    echo "package lexbase"
    echo
    grep '^const [A-Z][_A-Z0-9]* ' pkg/sql/parser/sql.go
  } >pkg/sql/lexbase/tokens.go
  {
    echo "// Code generated by make. DO NOT EDIT."
    echo "// GENERATED FILE DO NOT EDIT"
    echo
    echo "package scanner"
    echo
    echo "var tokenNames = map[int]string{"
    grep '^const [A-Z][_A-Z0-9]* ' pkg/sql/parser/sql.go | awk '{printf("%d: \"%s\",\n", $4, $2)}'
    echo "}"
  } >pkg/sql/scanner/token_names_test.go

  local tree=pkg/sql/sem/tree
  # shellcheck disable=SC2046
  go run ./pkg/sql/sem/tree/evalgen --out "$tree" \
    $(ls "$tree"/*.go | grep -v -e eval_expr_generated.go -e eval_op_generated.go)
}

gen_optgen() {
  build_tool optgen ./pkg/sql/opt/optgen/cmd/optgen
  build_tool langgen ./pkg/sql/opt/optgen/cmd/langgen
  local ops=(pkg/sql/opt/ops/*.opt)
  local norm=(pkg/sql/opt/norm/rules/*.opt)
  local xform=(pkg/sql/opt/xform/rules/*.opt)
  local defs=pkg/sql/opt/exec/factory.opt
  optgen -out pkg/sql/opt/operator.og.go ops "${ops[@]}"
  optgen -out pkg/sql/opt/rule_name.og.go rulenames "${ops[@]}" "${norm[@]}" "${xform[@]}"
  optgen -out pkg/sql/opt/memo/expr.og.go exprs "${ops[@]}"
  optgen -out pkg/sql/opt/norm/factory.og.go factory "${ops[@]}" "${norm[@]}"
  optgen -out pkg/sql/opt/xform/explorer.og.go explorer "${ops[@]}" "${xform[@]}"
  optgen -out pkg/sql/opt/exec/factory.og.go execfactory "$defs"
  optgen -out pkg/sql/opt/exec/explain/explain_factory.og.go execexplain "$defs"
  optgen -out pkg/sql/opt/exec/explain/plan_gist_factory.og.go execplangist "$defs"
  langgen -out pkg/sql/opt/optgen/lang/expr.og.go exprs pkg/sql/opt/optgen/lang/lang.opt
  langgen -out pkg/sql/opt/optgen/lang/operator.og.go ops pkg/sql/opt/optgen/lang/lang.opt
}

# Each line: package directory, output file and template. The output file
# name selects the generator inside execgen.
EXECGEN=(
  "pkg/col/coldata vec.eg.go -"
  "pkg/sql/colexec sort_partitioner.eg.go ../colexecbase/distinct_tmpl.go"
  "pkg/sql/colexec/colexecprojconst default_cmp_proj_const_op.eg.go ../colexecproj/default_cmp_proj_ops_tmpl.go"
  "pkg/sql/colexec and_or_projection.eg.go and_or_projection_tmpl.go"
  "pkg/sql/colexec hash_aggregator.eg.go hash_aggregator_tmpl.go"
  "pkg/sql/colexec is_null_ops.eg.go is_null_ops_tmpl.go"
  "pkg/sql/colexec ordered_synchronizer.eg.go ordered_synchronizer_tmpl.go"
  "pkg/sql/colexec pdqsort.eg.go pdqsort_tmpl.go"
  "pkg/sql/colexec rowtovec.eg.go rowtovec_tmpl.go"
  "pkg/sql/colexec select_in.eg.go select_in_tmpl.go"
  "pkg/sql/colexec sort.eg.go sort_tmpl.go"
  "pkg/sql/colexec sorttopk.eg.go sorttopk_tmpl.go"
  "pkg/sql/colexec substring.eg.go substring_tmpl.go"
  "pkg/sql/colexec values_differ.eg.go values_differ_tmpl.go"
  "pkg/sql/colexec vec_comparators.eg.go vec_comparators_tmpl.go"
  "pkg/sql/colconv datum_to_vec.eg.go datum_to_vec_tmpl.go"
  "pkg/sql/colconv vec_to_datum.eg.go vec_to_datum_tmpl.go"
  "pkg/sql/colexec/colexecprojconst proj_const_left_ops.eg.go proj_const_ops_tmpl.go"
  "pkg/sql/colexec/colexecprojconst proj_const_right_ops.eg.go proj_const_ops_tmpl.go"
  "pkg/sql/colexec/colexecprojconst proj_like_ops.eg.go proj_const_ops_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_any_not_null_agg.eg.go any_not_null_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_avg_agg.eg.go avg_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_bool_and_or_agg.eg.go bool_and_or_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_concat_agg.eg.go concat_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_count_agg.eg.go count_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_default_agg.eg.go default_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_min_max_agg.eg.go min_max_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_sum_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg hash_sum_int_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_any_not_null_agg.eg.go any_not_null_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_avg_agg.eg.go avg_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_bool_and_or_agg.eg.go bool_and_or_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_concat_agg.eg.go concat_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_count_agg.eg.go count_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_default_agg.eg.go default_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_min_max_agg.eg.go min_max_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_sum_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg ordered_sum_int_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_avg_agg.eg.go avg_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_bool_and_or_agg.eg.go bool_and_or_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_concat_agg.eg.go concat_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_count_agg.eg.go count_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_min_max_agg.eg.go min_max_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_sum_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecagg window_sum_int_agg.eg.go sum_agg_tmpl.go"
  "pkg/sql/colexec/colexecproj default_cmp_proj_op.eg.go default_cmp_proj_ops_tmpl.go"
  "pkg/sql/colexec/colexecproj proj_non_const_ops.eg.go proj_non_const_ops_tmpl.go"
  "pkg/sql/colexec/colexeccmp default_cmp_expr.eg.go default_cmp_expr_tmpl.go"
  "pkg/sql/colexec/colexecspan span_encoder.eg.go span_encoder_tmpl.go"
  "pkg/sql/colexec/colexechash hash_utils.eg.go hash_utils_tmpl.go"
  "pkg/sql/colexec/colexechash hashtable_distinct.eg.go hashtable_tmpl.go"
  "pkg/sql/colexec/colexechash hashtable_full_default.eg.go hashtable_tmpl.go"
  "pkg/sql/colexec/colexechash hashtable_full_deleting.eg.go hashtable_tmpl.go"
  "pkg/sql/colexec/colexecbase cast.eg.go cast_tmpl.go"
  "pkg/sql/colexec/colexecbase const.eg.go const_tmpl.go"
  "pkg/sql/colexec/colexecbase distinct.eg.go distinct_tmpl.go"
  "pkg/sql/colexec/colexecjoin crossjoiner.eg.go crossjoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin hashjoiner.eg.go hashjoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoinbase.eg.go mergejoinbase_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_exceptall.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_fullouter.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_inner.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_intersectall.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_leftanti.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_leftouter.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_leftsemi.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_rightanti.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_rightouter.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecjoin mergejoiner_rightsemi.eg.go mergejoiner_tmpl.go"
  "pkg/sql/colexec/colexecwindow first_value.eg.go first_last_nth_value_tmpl.go"
  "pkg/sql/colexec/colexecwindow lag.eg.go lead_lag_tmpl.go"
  "pkg/sql/colexec/colexecwindow last_value.eg.go first_last_nth_value_tmpl.go"
  "pkg/sql/colexec/colexecwindow lead.eg.go lead_lag_tmpl.go"
  "pkg/sql/colexec/colexecwindow min_max_removable_agg.eg.go min_max_removable_agg_tmpl.go"
  "pkg/sql/colexec/colexecwindow nth_value.eg.go first_last_nth_value_tmpl.go"
  "pkg/sql/colexec/colexecwindow ntile.eg.go ntile_tmpl.go"
  "pkg/sql/colexec/colexecwindow range_offset_handler.eg.go range_offset_handler_tmpl.go"
  "pkg/sql/colexec/colexecwindow rank.eg.go rank_tmpl.go"
  "pkg/sql/colexec/colexecwindow relative_rank.eg.go relative_rank_tmpl.go"
  "pkg/sql/colexec/colexecwindow row_number.eg.go row_number_tmpl.go"
  "pkg/sql/colexec/colexecwindow window_aggregator.eg.go window_aggregator_tmpl.go"
  "pkg/sql/colexec/colexecwindow window_framer.eg.go window_framer_tmpl.go"
  "pkg/sql/colexec/colexecwindow window_peer_grouper.eg.go window_peer_grouper_tmpl.go"
  "pkg/sql/colexec/colexecsel default_cmp_sel_ops.eg.go default_cmp_sel_ops_tmpl.go"
  "pkg/sql/colexec/colexecsel selection_ops.eg.go selection_ops_tmpl.go"
  "pkg/sql/colexec/colexecsel sel_like_ops.eg.go selection_ops_tmpl.go"
)

gen_execgen() {
  build_tool execgen ./pkg/sql/colexec/execgen/cmd/execgen
  build_tool goimports github.com/cockroachdb/gostdlib/x/tools/cmd/goimports
  local line dir out tmpl
  for line in "${EXECGEN[@]}"; do
    read -r dir out tmpl <<<"$line"
    if [[ $tmpl == - ]]; then
      execgen -fmt=false "$dir/$out" >"$dir/$out"
    else
      execgen -template "$dir/$tmpl" -fmt=false "$dir/$out" >"$dir/$out"
    fi
    goimports -w "$dir/$out"
  done
}

# Each line: package directory, source file, type and extra stringer flags.
STRINGER=(
  "pkg/base test_cluster_replication_mode.go TestClusterReplicationMode"
  "pkg/cli flags_util.go keyType --stringtovaluemapname=_keyTypes"
  "pkg/config field.go Field --linecomment"
  "pkg/kv/kvclient/kvcoord txn_coord_sender.go txnState"
  "pkg/kv/kvpb method.go Method --stringtovaluemapname=StringToMethodMap"
  "pkg/kv/kvpb errors.go ErrorDetailType"
  "pkg/kv/kvserver replica_raft.go refreshRaftReason"
  "pkg/kv/kvserver/closedts/sidetransport sender.go CantCloseReason"
  "pkg/multitenant/tenantcapabilities capabilities.go ID --linecomment --stringtovaluemapname=stringToCapabilityIDMap --enumvaluesslicename=IDs"
  "pkg/sql txn_state.go txnEventType"
  "pkg/sql txn_state.go txnType"
  "pkg/sql txn_state.go advanceCode"
  "pkg/sql distsql_physical_planner.go NodeStatus"
  "pkg/sql/catalog/catalogkeys keys.go CommentType"
  "pkg/sql/catalog/catpb privilege.go PrivilegeDescVersion"
  "pkg/sql/catalog/descpb structured.go FormatVersion"
  "pkg/sql/colfetcher cfetcher.go fetcherState"
  "pkg/sql/execinfra processorsbase.go procState"
  "pkg/sql/execinfra base.go ConsumerStatus"
  "pkg/sql/opt/optgen/lang scanner.go Token"
  "pkg/sql/opt/optgen/lang operator.og.go Operator"
  "pkg/sql/pgwire/pgwirebase encoding.go PGNumericSign"
  "pkg/sql/pgwire/pgwirebase encoding.go FormatCode"
  "pkg/sql/pgwire/pgwirebase msg.go ClientMessageType"
  "pkg/sql/pgwire/pgwirebase msg.go ServerMessageType"
  "pkg/sql/pgwire/pgwirebase msg.go ServerErrFieldType"
  "pkg/sql/pgwire/pgwirebase msg.go PrepareType"
  "pkg/sql/roleoption role_option.go Option -linecomment"
  "pkg/sql/schemachange alter_column_type.go ColumnConversionKind --trimprefix ColumnConversion"
  "pkg/sql/schemachanger/rel system_attributes.go systemAttribute"
  "pkg/sql/schemachanger/rel/internal/comparetest schema.go testAttr"
  "pkg/sql/schemachanger/rel/internal/cyclegraphtest schema.go testAttr"
  "pkg/sql/schemachanger/rel/internal/entitynodetest schema.go testAttr"
  "pkg/sql/schemachanger/scop ops.go Type"
  "pkg/sql/schemachanger/scop phase.go Phase"
  "pkg/sql/schemachanger/scplan/internal/scgraph edge.go DepEdgeKind"
  "pkg/sql/schemachanger/screl attr.go Attr"
  "pkg/sql/schemachanger/sctest framework.go stageExecType"
  "pkg/sql/sem/tree create.go CreateTypeVariety"
  "pkg/sql/sem/tree stmt.go StatementReturnType"
  "pkg/sql/sem/tree stmt.go StatementType"
  "pkg/util/timeutil/pgdate fields.go field"
  "pkg/workload/schemachange optype.go opType"
)

gen_stringer() {
  build_tool tinystringer ./pkg/cmd/tinystringer
  local line dir src typ args lower
  for line in "${STRINGER[@]}"; do
    read -r dir src typ args <<<"$line"
    lower=$(tr '[:upper:]' '[:lower:]' <<<"$typ")
    # shellcheck disable=SC2086
    tinystringer -output="$dir/${lower}_string.go" -type="$typ" $args "$dir/$src"
  done
  tinystringer -output=pkg/sql/opt/rule_name_string.go -type=RuleName \
    pkg/sql/opt/rule_name.go pkg/sql/opt/rule_name.og.go
  # The Type of pkg/util/encoding is an alias that stringer cannot follow.
  local tmp
  tmp=$(mktemp -d)
  sed -e 's/type Type encodingtype.T/type Type int/' pkg/util/encoding/encoding.go >"$tmp/encoding.go"
  tinystringer -output=pkg/util/encoding/type_string.go -type=Type "$tmp/encoding.go"
  rm -rf "$tmp"
}

gen_mocks() {
  build_tool mockgen github.com/golang/mock/mockgen
  local p=github.com/cockroachdb/cockroach/pkg
  mockgen -package kvcoord -self_package $p/kv/kvclient/kvcoord \
    -destination pkg/kv/kvclient/kvcoord/mocks_generated_test.go $p/kv/kvclient/kvcoord Transport
  mockgen -package rangecachemock -self_package $p/kv/kvclient/rangecache/rangecachemock \
    -destination pkg/kv/kvclient/rangecache/rangecachemock/mocks_generated.go $p/kv/kvclient/rangecache RangeDescriptorDB
  mockgen -package rangefeed -self_package $p/kv/kvclient/rangefeed \
    -destination pkg/kv/kvclient/rangefeed/mocks_generated_test.go $p/kv/kvclient/rangefeed DB
  mockgen -package kvpbmock -self_package $p/kv/kvpb/kvpbmock \
    -destination pkg/kv/kvpb/kvpbmock/mocks_generated.go $p/kv/kvpb \
    InternalClient,Internal_RangeFeedClient,Internal_MuxRangeFeedClient
  mockgen -package rpc -self_package $p/rpc \
    -destination pkg/rpc/mocks_generated_test.go $p/rpc Dialbacker
  mockgen -package certmgr \
    -destination pkg/security/certmgr/mocks_generated_test.go $p/security/certmgr Cert
  mockgen -package scexec_test \
    -destination pkg/sql/schemachanger/scexec/mocks_generated_test.go $p/sql/schemachanger/scexec \
    Catalog,Dependencies,Backfiller,Merger,BackfillerTracker,IndexSpanSplitter,PeriodicProgressFlusher
  mockgen -package log -self_package $p/util/log -mock_names TestingLogSink=MockLogSink \
    -destination pkg/util/log/mocks_generated_test.go $p/util/log TestingLogSink
}

EVENTPB_PROTOS=(
  pkg/util/log/logpb/event.proto
  pkg/util/log/eventpb/events.proto
  pkg/util/log/eventpb/debug_events.proto
  pkg/util/log/eventpb/zone_events.proto
  pkg/util/log/eventpb/ddl_events.proto
  pkg/util/log/eventpb/misc_sql_events.proto
  pkg/util/log/eventpb/privilege_events.proto
  pkg/util/log/eventpb/role_events.proto
  pkg/util/log/eventpb/session_events.proto
  pkg/util/log/eventpb/sql_audit_events.proto
  pkg/util/log/eventpb/cluster_events.proto
  pkg/util/log/eventpb/job_events.proto
  pkg/util/log/eventpb/health_events.proto
  pkg/util/log/eventpb/storage_events.proto
  pkg/util/log/eventpb/telemetry.proto
)

gen_misc() {
  build_tool goimports github.com/cockroachdb/gostdlib/x/tools/cmd/goimports
  build_tool crlfmt github.com/cockroachdb/crlfmt
  build_tool go_generics github.com/mmatczuk/go_generics/cmd/go_generics

  go run ./pkg/kv/kvpb/gen --filename pkg/kv/kvpb/batch_generated.go pkg/kv/kvpb/*.pb.go

  (cd pkg/kv/kvserver/concurrency && ../../../util/interval/generic/gen.sh '*keyLocks' concurrency)
  (cd pkg/kv/kvserver/spanlatch && ../../../util/interval/generic/gen.sh '*latch' spanlatch)
  (cd pkg/spanconfig/spanconfigstore && ../../util/interval/generic/gen.sh '*entry' spanconfigstore)
  (cd pkg/util/interval/generic && ./gen.sh '*example' generic)

  (cd pkg/sql && ../util/fsm/gen/reports.sh TxnStateTransitions stateNoTxn)

  local iface out
  for iface in ApplicationLayerInterface:app StorageLayerInterface:storage \
    TenantControlInterface:tenant_control TestServerController:ts_control; do
    out=pkg/testutils/serverutils/${iface#*:}_forwarder_generated.go
    go run ./pkg/testutils/serverutils/fwgen -type wrap -nodef \
      -output "$out" pkg/testutils/serverutils/api.go "${iface%%:*}"
    goimports -w "$out"
  done

  local logproto=pkg/util/log/logpb/log.proto
  go run ./pkg/util/log/gen "$logproto" log_channels.go pkg/util/log/log_channels_generated.go
  go run ./pkg/util/log/gen "$logproto" channel.go pkg/util/log/channel/channel_generated.go
  go run ./pkg/util/log/gen "$logproto" severity.go pkg/util/log/severity/severity_generated.go
  awk -f pkg/util/log/eventpb/eventpbgen/extract_log_channels.awk <"$logproto" \
    >pkg/util/log/eventpb/eventpbgen/log_channels_generated.go
  go run ./pkg/util/log/eventpb/eventpbgen eventlog_channels_go "${EVENTPB_PROTOS[@]}" \
    >pkg/util/log/eventpb/eventlog_channels_generated.go
  go run ./pkg/util/log/eventpb/eventpbgen --excluded-events=CommonEventDetails json_encode_go \
    "${EVENTPB_PROTOS[@]}" >pkg/util/log/eventpb/json_encode_generated.go
  go run ./pkg/util/log/eventpb/eventpbgen --package=logpb json_encode_go \
    pkg/util/log/logpb/event.proto >pkg/util/log/logpb/json_encode_generated.go

  go run ./pkg/util/timeutil/gen --filename=pkg/util/timeutil/lowercase_timezones_generated.go \
    --zoneinfo="$(go env GOROOT)/lib/time/zoneinfo.zip" --crlfmt="$bin/crlfmt"
}

gen_schemachanger() {
  local scop=pkg/sql/schemachanger/scop scpb=pkg/sql/schemachanger/scpb
  local kind file
  for kind in ImmediateMutation:immediate_mutation DeferredMutation:deferred_mutation \
    Backfill:backfill Validation:validation; do
    file=${kind#*:}
    go run -tags generator ./$scop/generate_visitor.go scop "${kind%%:*}" \
      "$scop/$file.go" "$scop/${file}_visitor_generated.go"
  done
  go run -tags generator ./$scpb/element_generator.go --in $scpb/elements.proto --out $scpb/elements_generated.go
  go run -tags generator ./$scpb/element_uml_generator.go --out $scpb/uml/table.puml

  local tests=() t
  for t in EndToEndSideEffects ExecuteWithDMLInjection GenerateSchemaChangeCorpus Pause PauseMixedVersion Rollback; do
    tests+=(--tests "$t")
  done
  go run ./pkg/sql/schemachanger/sctest/sctestgen --package schemachanger "${tests[@]}" \
    --new-cluster-factory 'sctest.SingleNodeTestClusterFactory{}' \
    --out pkg/sql/schemachanger/sctest_generated_test.go \
    pkg/sql/schemachanger/testdata/end_to_end/*/*.definition
  go run ./pkg/sql/schemachanger/sctest/sccomparatortestgen \
    -out-file=pkg/sql/schemachanger/comparator_generated_test.go
}

gen_logictest() {
  go run ./pkg/cmd/generate-logictest -out-dir="$root"
}

gen_docs() {
  install_protoc
  proto_include
  build_tool protoc-gen-doc github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
  build_tool docgen ./pkg/cmd/docgen
  go build -o "$bin/cockroach" ./pkg/cmd/cockroach-oss

  local d=docs/generated
  cockroach gen settings-list --show-class --format=rawhtml --class-header-label 'Supported Deployments' \
    --class-labels 'Dedicated/Self-Hosted,Serverless/Dedicated/Self-Hosted (read-only),Serverless/Dedicated/Self-Hosted' \
    >$d/settings/settings.html
  cockroach gen settings-list --show-class --without-system-only >$d/settings/settings-for-tenants.txt
  cockroach gen metric-list --format=unnumbered-html --logtostderr=NONE >$d/metrics/metrics.html

  docgen functions $d/sql --quiet
  rm -f $d/sql/bnf/*.bnf
  docgen grammar bnf $d/sql/bnf --quiet --addr pkg/sql/parser/sql.y
  docgen logformats $d/logformats.md
  docgen http --protoc "$PROTOC" --gendoc "$bin/protoc-gen-doc" --out $d/http \
    --protoc-flags "-Ipkg -I$bin/proto-include -I$PROTOC_INCLUDE server/serverpb/status.proto server/serverpb/admin.proto server/status/statuspb/status.proto"
  go run ./pkg/util/log/gen pkg/util/log/logpb/log.proto logging.md $d/logging.md
  go run ./pkg/util/log/logconfig/gen <pkg/util/log/logconfig/config.go >$d/logsinks.md
  go run ./pkg/util/log/eventpb/eventpbgen eventlog.md "${EVENTPB_PROTOS[@]}" >$d/eventlog.md
}

steps=("$@")
if [[ ${#steps[@]} -eq 0 ]]; then
  steps=(proto parser optgen execgen stringer mocks misc schemachanger logictest docs)
fi
for step in "${steps[@]}"; do
  "gen_$step"
done
