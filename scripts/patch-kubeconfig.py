#!/usr/bin/env python3
"""Patch kubeconfig for use inside Docker Desktop using line-by-line text manipulation.
Preserves all base64 credential fields exactly (avoids yaml.dump re-encoding issues).
Changes:
- Replace 127.0.0.1 with host.docker.internal in server URLs
- Replace certificate-authority-data line with insecure-skip-tls-verify: true
"""
import sys
import re

src, dst = sys.argv[1], sys.argv[2]

with open(src) as f:
    lines = f.readlines()

out = []
for line in lines:
    # Replace server address
    line = re.sub(r'(server:\s*)https://127\.0\.0\.1', r'\1https://host.docker.internal', line)
    # Replace certificate-authority-data with insecure-skip-tls-verify
    if re.match(r'\s*certificate-authority-data:', line):
        indent = len(line) - len(line.lstrip())
        line = ' ' * indent + 'insecure-skip-tls-verify: true\n'
    # Remove certificate-authority (file path variant)
    elif re.match(r'\s*certificate-authority:', line):
        continue
    out.append(line)

with open(dst, 'w') as f:
    f.writelines(out)
