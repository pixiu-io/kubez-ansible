#!/bin/bash
# docker/smoke-test.sh — kubez-ansible 镜像运行时契约冒烟断言（容器内 root 执行）
# 语义：只做解析与探测，不做任何真实部署；写入仅限 /etc/kubez、/tmp、/var/log
# 运行：docker run --rm -v "$PWD/docker:/smoke:ro" <image> bash /smoke/smoke-test.sh
# 资产：/smoke/smoke-assets/（pkg-baseline.txt / pkg-baseline-arm64.txt / pkg-allowlist.txt / pkg-guard.txt）
set -euo pipefail

FAILS=0
pass() { echo "PASS  $*"; }
fail() { echo "FAIL  $*"; FAILS=$((FAILS + 1)); }
warn() { echo "WARN  $*"; }
check_bin() { if command -v "$1" >/dev/null 2>&1; then pass "bin: $1"; else fail "bin: $1 缺失"; fi; }
check_run() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi; }

# 冒烟资产目录（CI 经 -v "$PWD/docker:/smoke:ro" 挂入；本地演练可用 SMOKE_ASSETS 覆盖）
ASSETS_DIR="${SMOKE_ASSETS:-/smoke/smoke-assets}"

# 临时文件统一放 /tmp（不写镜像内安装树），退出时清理
TMPD=$(mktemp -d /tmp/kubez-smoke.XXXXXX)
PROBE="$TMPD/smoke-syntax-probe.yml"
trap 'rm -rf "${TMPD:-}"' EXIT

echo "=== [0] 版本信息 ==="
ansible --version || true
ansible-galaxy --version || true

echo "=== [1] kubez-ansible CLI 可用 ==="
check_bin kubez-ansible
check_run "kubez-ansible --help 退出码 0" kubez-ansible --help
check_run "kubez-ansible bash-completion 退出码 0" kubez-ansible bash-completion

echo "=== [2] 必需二进制 / 运行时前提 ==="
for b in bash python3 getopt ssh sftp sshpass ansible ansible-playbook ansible-galaxy cp mkdir; do
  check_bin "$b"
done
check_run "python3 可 import yaml, jinja2" python3 -c 'import yaml, jinja2'
check_run "/var/log/ansible.log 可写（CLI 固定 ANSIBLE_LOG_PATH）" bash -c ': >> /var/log/ansible.log'

echo "=== [3] collections 在位（静态扫描唯一非 builtin 依赖：ansible.posix；community.general 为显式钉版安装） ==="
COLL_LIST=$(ansible-galaxy collection list 2>/dev/null || true)
for c in ansible.posix community.general; do
  if printf '%s\n' "$COLL_LIST" | awk '{print $1}' | grep -qx "$c"; then
    pass "collection: $c"
  else
    fail "collection: $c 缺失（ansible.posix: authorized_key/selinux/sysctl 无法解析；community.general: 钉版契约缺失）"
  fi
done

echo "=== [4] 安装布局（BASEDIR 解析规则与 tools/kubez-ansible 一致） ==="
BIN_REAL=$(python3 -c "import os;print(os.path.realpath('$(command -v kubez-ansible)'))")
BIN_DIR=$(dirname "$BIN_REAL")
if   [ "$BIN_DIR" = "/usr/bin" ];       then BASE=/usr/share/kubez-ansible
elif [ "$BIN_DIR" = "/usr/local/bin" ]; then BASE=/usr/local/share/kubez-ansible
else BASE=$(dirname "$BIN_DIR"); fi
for p in ansible/site.yml ansible/kubernetes-hosts.yml ansible/authorized-key.yml ansible/post-deploy.yml \
         ansible/gather-facts.yml ansible/inventory/all-in-one ansible/inventory/multinode \
         ansible/group_vars/all.yml ansible/roles ansible/library ansible/filter_plugins/to_socket.py; do
  if [ -e "$BASE/$p" ]; then pass "layout: $p"; else fail "layout: $p 缺失（BASE=${BASE}）"; fi
done

echo "=== [5] 运行前提：/etc/kubez/globals.yml（CLI 无条件 -e @ 拼装）+ multinode（start.sh 强制 -i） ==="
mkdir -p /etc/kubez
if [ ! -s /etc/kubez/globals.yml ]; then
  printf -- '---\n' > /etc/kubez/globals.yml
  warn "已生成临时最小 /etc/kubez/globals.yml（真实容器由 /configs/globals.yml 提供）"
fi
INV=/etc/kubez/multinode
if [ ! -s "$INV" ]; then
  if [ -f "$BASE/ansible/inventory/multinode" ]; then
    cp "$BASE/ansible/inventory/multinode" "$INV"
    warn "已用镜像内模板生成 /etc/kubez/multinode（真实容器由 /configs/multinode 提供）"
  else
    INV="$BASE/ansible/inventory/all-in-one"
    warn "multinode 模板缺失，回退 -i $INV"
  fi
fi

echo "=== [6] 入口 playbook syntax-check（直接 ansible-playbook，逐入口定位） ==="
for pb in site.yml kubernetes-hosts.yml authorized-key.yml post-deploy.yml; do
  if out=$(ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook --syntax-check \
             -i "$INV" -e @/etc/kubez/globals.yml -e CONFIG_DIR=/etc/kubez \
             "$BASE/ansible/$pb" 2>&1); then
    pass "syntax-check: $pb"
  else
    fail "syntax-check: $pb"; echo "$out" | tail -5
  fi
done

echo "=== [7] CLI 等价干跑（生产形态参数，EXTRA_OPTS 注入 --syntax-check） ==="
if out=$(EXTRA_OPTS="--syntax-check" kubez-ansible -i "$INV" deploy 2>&1); then
  pass "kubez-ansible deploy --syntax-check（CLI 全链路：BASEDIR/inventory/globals 拼装）"
else
  fail "CLI 干跑失败"; echo "$out" | tail -8
fi

echo "=== [8] role 任务影子探针（include_tasks 是动态的，入口 syntax-check 覆盖不到 roles/*/tasks/） ==="
# 探针写 /tmp；library/filter_plugins 经 ANSIBLE_* 环境变量指向安装树（等价于 playbook 同级自动发现）
if { echo '---'
     echo '- name: smoke probe'
     echo '  hosts: localhost'
     echo '  gather_facts: false'
     echo '  tasks:'
     find "$BASE/ansible/roles" -path '*/tasks/*' \( -name '*.yml' -o -name '*.yaml' \) | sort |
       while IFS= read -r f; do printf '    - import_tasks: %s\n' "$f"; done
   } > "$PROBE" 2>/dev/null; then
  if out=$(ANSIBLE_HOST_KEY_CHECKING=False \
             ANSIBLE_LIBRARY="$BASE/ansible/library" \
             ANSIBLE_FILTER_PLUGINS="$BASE/ansible/filter_plugins" \
             ansible-playbook --syntax-check -i "$INV" "$PROBE" 2>&1); then
    pass "shadow probe：$(grep -c 'import_tasks' "$PROBE") 个 role 任务文件解析通过（含自定义模块与 ansible.posix 短名解析）"
  else
    fail "shadow probe（报 couldn't resolve module/action 时对照 [3]）"; echo "$out" | tail -10
  fi
else
  fail "无法写入影子探针 $PROBE"
fi

echo "=== [9] 条件契约（仅容器内 all-in-one 形态需要；缺失记 WARN，不阻断） ==="
python3 -c 'import requests' 2>/dev/null && pass "python3-requests" \
  || warn "缺 python3-requests：多节点部署无影响；容器内 all-in-one 的 gpg_key 模块会失败"
command -v gpg >/dev/null 2>&1 && pass "gpg" \
  || warn "缺 gpg：同上（gpg_key 的 --dearmor 依赖）"
command -v kubectl >/dev/null 2>&1 && pass "kubectl" \
  || warn "缺 kubectl：容器内 all-in-one 形态不支持（当前属预期）"

echo "=== [10] ansible-core 版本一致性（dpkg 实况 vs 基线文件） ==="
ARCH=$(dpkg --print-architecture 2>/dev/null || true)
case "$ARCH" in
  amd64) BASELINE_FILE="$ASSETS_DIR/pkg-baseline.txt" ;;
  arm64) BASELINE_FILE="$ASSETS_DIR/pkg-baseline-arm64.txt" ;;
  *)     BASELINE_FILE="$ASSETS_DIR/pkg-baseline.txt"
         [ -z "$ARCH" ] || warn "未知架构 '$ARCH'，回退 amd64 基线" ;;
esac
if [ ! -f "$BASELINE_FILE" ]; then
  fail "基线文件缺失：${BASELINE_FILE}（ASSETS_DIR=${ASSETS_DIR}）"
else
  EXPECTED_CORE=$(awk '$1=="ansible-core"{print $2}' "$BASELINE_FILE")
  ACTUAL_CORE=$(dpkg-query -W -f='${Version}' ansible-core 2>/dev/null || true)
  if [ -n "$EXPECTED_CORE" ] && [ "$ACTUAL_CORE" = "$EXPECTED_CORE" ]; then
    pass "ansible-core 版本一致：${ACTUAL_CORE}（基线 $(basename "$BASELINE_FILE")，arch=${ARCH}）"
  else
    fail "ansible-core 版本不一致：实际='$ACTUAL_CORE' 基线='$EXPECTED_CORE'"
  fi
fi

echo "=== [11] dpkg 包集 diff（新增为空；移除 ⊆ 白名单；移除 ∩ 禁移除集 = ∅） ==="
ALLOWLIST_FILE="$ASSETS_DIR/pkg-allowlist.txt"
GUARD_FILE="$ASSETS_DIR/pkg-guard.txt"
if [ ! -f "$BASELINE_FILE" ] || [ ! -f "$ALLOWLIST_FILE" ] || [ ! -f "$GUARD_FILE" ]; then
  fail "dpkg diff 资产缺失（基线=$BASELINE_FILE 白名单=$ALLOWLIST_FILE 禁移除=${GUARD_FILE}）"
else
  dpkg-query -W -f='${db:Status-Abbrev} ${Package}\n' 2>/dev/null | awk '$1=="ii"{print $2}' | sort > "$TMPD/now.txt"
  awk '{print $1}' "$BASELINE_FILE" | sort > "$TMPD/base.txt"
  awk '!/^[[:space:]]*#/ && NF {print $1}' "$ALLOWLIST_FILE" | sort -u > "$TMPD/allow.txt"
  awk '!/^[[:space:]]*#/ && NF {print $1}' "$GUARD_FILE" | sort -u > "$TMPD/guard.txt"

  # comm 要求两侧同序：removed = 基线有而今无；added = 今有而基线无
  comm -23 "$TMPD/base.txt" "$TMPD/now.txt" > "$TMPD/removed.txt"
  comm -13 "$TMPD/base.txt" "$TMPD/now.txt" > "$TMPD/added.txt"
  n_removed=$(wc -l < "$TMPD/removed.txt")
  n_added=$(wc -l < "$TMPD/added.txt")

  if [ "$n_added" -eq 0 ]; then
    pass "dpkg diff：基线外新增包 0 个"
  else
    fail "dpkg diff：出现 $n_added 个基线外新增包（前 10）："
    head -10 "$TMPD/added.txt" | sed 's/^/        + /'
  fi

  outside=$(comm -23 "$TMPD/removed.txt" "$TMPD/allow.txt" || true)
  if [ -z "$outside" ]; then
    pass "dpkg diff：移除 $n_removed 个包，全部 ⊆ 白名单"
  else
    fail "dpkg diff：移除集含白名单外包 $(printf '%s\n' "$outside" | wc -l | tr -d ' ') 个（前 10）："
    printf '%s\n' "$outside" | head -10 | sed 's/^/        - /'
  fi

  guard_hit=$(comm -12 "$TMPD/removed.txt" "$TMPD/guard.txt" || true)
  if [ -z "$guard_hit" ]; then
    pass "dpkg diff：禁移除集 ∩ 移除集 = ∅"
  else
    fail "dpkg diff：禁移除包被移除 $(printf '%s\n' "$guard_hit" | wc -l | tr -d ' ') 个（前 10）："
    printf '%s\n' "$guard_hit" | head -10 | sed 's/^/        ! /'
  fi
fi

echo "=== [12] collections 钉版断言（MANIFEST.json）+ pip 元数据可得性（WARN） ==="
COLL_ROOT=/usr/share/ansible/collections/ansible_collections
check_collection_pin() {
  local name="$1" want="$2" path="$3" got=""
  if [ ! -f "$path/MANIFEST.json" ]; then
    fail "collection ${name}：缺 $path/MANIFEST.json"
    return 0
  fi
  got=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["collection_info"]["version"])' \
          "$path/MANIFEST.json" 2>/dev/null || true)
  if [ "$got" = "$want" ]; then
    pass "collection $name 钉版 = $got"
  else
    fail "collection $name 版本不符：实际='$got' 期望='$want'"
  fi
}
check_collection_pin ansible.posix 1.5.4 "$COLL_ROOT/ansible/posix"
check_collection_pin community.general 8.3.0 "$COLL_ROOT/community/general"

if python3 -c 'import pkg_resources' 2>/dev/null; then
  if python3 -c 'import pkg_resources; pkg_resources.get_distribution("kubez-ansible")' 2>/dev/null; then
    pass "pkg_resources 可用且可查得 kubez-ansible 分发元数据"
  else
    warn "pkg_resources 可用但查不到 kubez-ansible 分发元数据（pip 安装产物异常？不阻断）"
  fi
else
  warn "python3 无法 import pkg_resources（python3-pkg-resources 已随瘦身移除，属预期，不阻断）"
fi

echo
echo "=== SUMMARY: FAIL=${FAILS}（0 通过；WARN 不计数） ==="
[ "$FAILS" -eq 0 ]
