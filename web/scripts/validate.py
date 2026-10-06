#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
前端骨架静态校验（无需安装依赖）：
1. 文件清单完整性
2. import 引用链（@/ 别名 → 实际文件）
3. Vue SFC 结构（script/template/style 配对）
4. package.json 依赖与源码 import 一致性
用法: python scripts/validate.py
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # web/
SRC = os.path.join(ROOT, "src")
errors, warns = [], []

# ---------- 1. 文件清单 ----------
expected = [
    "package.json", "vite.config.ts", "tsconfig.json", "index.html",
    ".env.development", ".env.production", "README.md", ".gitignore",
    "src/main.ts", "src/App.vue", "src/vite-env.d.ts",
    "src/router/index.ts", "src/stores/auth.ts",
    "src/api/http.ts", "src/api/auth.ts", "src/api/logs.ts", "src/api/devices.ts",
    "src/utils/password.ts", "src/utils/session.ts", "src/utils/security.ts", "src/utils/audit.ts",
    "src/directives/index.ts",
    "src/components/layout/AppLayout.vue",
    "src/components/security/PasswordStrength.vue",
    "src/views/login/LoginView.vue",
    "src/views/dashboard/DashboardView.vue",
    "src/views/logs/LogSearchView.vue",
    "src/views/stats/StatsView.vue",
    "src/views/devices/DevicesView.vue",
    "src/views/retention/RetentionView.vue",
    "src/views/settings/SettingsView.vue",
    "src/views/error/Error403.vue", "src/views/error/Error404.vue",
    "src/styles/global.css", "src/types/index.ts",
]
print(f"[1/4] 文件清单: 期望 {len(expected)} 个")
for f in expected:
    p = os.path.join(ROOT, f.replace("/", os.sep))
    if not os.path.isfile(p):
        errors.append(f"缺失文件: {f}")

# 额外发现文件（未被期望但存在）
found = []
for root_dir, _, files in os.walk(ROOT):
    if "node_modules" in root_dir:
        continue
    for fn in files:
        rel = os.path.relpath(os.path.join(root_dir, fn), ROOT).replace(os.sep, "/")
        if rel not in expected and not rel.startswith("scripts/"):
            found.append(rel)
if found:
    warns.append(f"额外文件: {found}")

# ---------- 2. import 引用链 ----------
print("[2/4] import 引用链检查")
import_re = re.compile(r"""from\s+['"]([^'"]+)['"]|import\s+['"]([^'"]+)['"]""")
alias_re = re.compile(r"^@/(.+)$")
def resolve_exists(base: str, spec: str) -> bool:
    """解析模块引用是否存在（支持目录 index 解析）
    @/x → 相对 src/ 根；./x → 相对当前文件目录
    """
    if spec.startswith("@/"):
        target = os.path.normpath(os.path.join(SRC, spec[2:]))
    elif spec.startswith("."):
        target = os.path.normpath(os.path.join(base, spec))
    else:
        return False
    candidates = [target, target + ".ts", target + ".vue", target + ".tsx"]
    # 目录索引（index.ts / index.vue）
    for idx in ("index.ts", "index.vue", "index.tsx", "index.js", "index.mjs"):
        candidates.append(os.path.join(target, idx))
    return any(os.path.isfile(c) for c in candidates)


for root_dir, _, files in os.walk(SRC):
    for fn in files:
        if not (fn.endswith(".ts") or fn.endswith(".vue")):
            continue
        p = os.path.join(root_dir, fn)
        rel = os.path.relpath(p, ROOT).replace(os.sep, "/")
        text = open(p, encoding="utf-8").read()
        for m in import_re.finditer(text):
            spec = m.group(1) or m.group(2)
            if not spec:
                continue
            # 裸模块（第三方包）跳过本地解析，由第 4 步依赖一致性检查
            if not (spec.startswith(".") or spec.startswith("@/")):
                continue
            base = os.path.dirname(p)
            if not resolve_exists(base, spec):
                errors.append(f"{rel}: 引用不存在 {spec}")

# ---------- 3. Vue SFC 结构 ----------
print("[3/4] Vue SFC 结构检查")
for root_dir, _, files in os.walk(SRC):
    for fn in files:
        if not fn.endswith(".vue"):
            continue
        p = os.path.join(root_dir, fn)
        rel = os.path.relpath(p, ROOT).replace(os.sep, "/")
        text = open(p, encoding="utf-8").read()
        for tag in ("script", "template", "style"):
            opens = text.count(f"<{tag}")
            closes = text.count(f"</{tag}>")
            if opens != closes:
                errors.append(f"{rel}: <{tag}> 标签不配对 ({opens} vs {closes})")
        if "<template>" not in text:
            warns.append(f"{rel}: 无 template 块")

# ---------- 4. 依赖一致性 ----------
print("[4/4] 依赖与源码 import 一致性")
pkg = json.load(open(os.path.join(ROOT, "package.json"), encoding="utf-8"))
deps = set(pkg.get("dependencies", {})) | set(pkg.get("devDependencies", {}))
used = set()
for root_dir, _, files in os.walk(SRC):
    for fn in files:
        if not (fn.endswith(".ts") or fn.endswith(".vue")):
            continue
        text = open(os.path.join(root_dir, fn), encoding="utf-8").read()
        for m in import_re.finditer(text):
            spec = m.group(1) or m.group(2)
            if spec and not spec.startswith(".") and not spec.startswith("@/"):
                # scoped 包（@scope/pkg）保留两级，普通包取首段
                if spec.startswith("@") and "/" in spec:
                    used.add(spec.split("/")[0] + "/" + spec.split("/")[1])
                else:
                    used.add(spec.split("/")[0])
for u in sorted(used):
    if u not in deps and u not in ("vue",):
        if u == "vue":
            continue
        warns.append(f"源码引用未在 package.json 声明: {u}")

print("\n=================== 结果 ===================")
if errors:
    print(f"错误 {len(errors)} 项:")
    for e in errors:
        print(f"  [ERR] {e}")
    sys.exit(1)
print("无错误。")
if warns:
    print(f"警告 {len(warns)} 项:")
    for w in warns:
        print(f"  [WARN] {w}")
else:
    print("无警告。")
sys.exit(0)
