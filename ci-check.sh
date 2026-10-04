#!/bin/bash
# SFI-MITM iOS 编译自检脚本
# 用途：推送代码后自动等待 CI、检查状态、下载日志/产物
# 用法：./ci-check.sh [run_id]  （不传则自动取最新一次 ios-build）

set -euo pipefail

REPO="lambret-1/SFI-MITM"
BRANCH="sing-box-for-apple"
WORKFLOW="ios-build.yml"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

# 获取最新 run id
get_latest_run() {
  gh api "repos/$REPO/actions/workflows/$WORKFLOW/runs?branch=$BRANCH&per_page=1" \
    --jq '.workflow_runs[0].id'
}

# 等待 CI 完成（最多 30 分钟）
wait_for_ci() {
  local run_id=$1
  local elapsed=0
  echo -e "${YELLOW}等待 CI 运行 #$run_id 完成...${NC}"
  while true; do
    local status=$(gh api "repos/$REPO/actions/runs/$run_id" --jq '.status')
    if [ "$status" = "completed" ]; then
      local conclusion=$(gh api "repos/$REPO/actions/runs/$run_id" --jq '.conclusion')
      echo -e "${GREEN}CI 完成，结论: $conclusion${NC}"
      echo "$conclusion"
      return 0
    fi
    if [ $elapsed -ge 1800 ]; then
      echo -e "${RED}超时（30分钟）${NC}"
      echo "timeout"
      return 1
    fi
    sleep 30
    elapsed=$((elapsed + 30))
    echo "  已等待 ${elapsed}s，状态: $status"
  done
}

# 下载失败日志
download_failure_logs() {
  local run_id=$1
  echo -e "${YELLOW}下载失败 Job 日志...${NC}"
  local job_id=$(gh api "repos/$REPO/actions/runs/$run_id/jobs" --jq '.jobs[] | select(.conclusion=="failure") | .id' | head -1)
  if [ -n "$job_id" ]; then
    gh api "repos/$REPO/actions/jobs/$job_id/logs" > "build_failure_${run_id}.log" 2>/dev/null || true
    echo -e "${RED}=== 编译错误 ===${NC}"
    grep -n "error:" "build_failure_${run_id}.log" 2>/dev/null | head -40 || echo "未提取到 error: 行"
    echo -e "${YELLOW}日志已保存: build_failure_${run_id}.log${NC}"
  fi
}

# 下载 IPA 产物
download_artifact() {
  local run_id=$1
  echo -e "${YELLOW}下载 IPA 产物...${NC}"
  gh run download "$run_id" --repo "$REPO" -D "ci_artifacts_${run_id}" 2>/dev/null || true
  ls -lh "ci_artifacts_${run_id}/"*.ipa 2>/dev/null && echo -e "${GREEN}IPA 下载完成${NC}" || echo "未找到 IPA 产物"
}

# 主流程
RUN_ID=${1:-$(get_latest_run)}
if [ -z "$RUN_ID" ] || [ "$RUN_ID" = "null" ]; then
  echo -e "${RED}未找到 CI 运行记录${NC}"
  exit 1
fi

echo "========================================"
echo " SFI-MITM iOS 编译自检"
echo " 仓库: $REPO"
echo " 分支: $BRANCH"
echo " Run ID: $RUN_ID"
echo "========================================"

CONCLUSION=$(wait_for_ci "$RUN_ID")

if [ "$CONCLUSION" = "success" ]; then
  echo -e "${GREEN}✅ 构建成功${NC}"
  download_artifact "$RUN_ID"
  exit 0
elif [ "$CONCLUSION" = "failure" ]; then
  echo -e "${RED}❌ 构建失败${NC}"
  download_failure_logs "$RUN_ID"
  exit 1
else
  echo -e "${RED}⚠️  CI 状态异常: $CONCLUSION${NC}"
  exit 2
fi
