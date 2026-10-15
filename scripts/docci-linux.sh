#!/usr/bin/env bash
# Run a page on Linux, from a Mac, the way CI will.
#
#   scripts/docci-linux.sh docs/my-page.md
#
# Why this exists: a page written on a Mac can pass locally and fail in CI over
# file ownership alone, because Docker Desktop does not enforce the host uid and
# a Linux daemon does. Reproducing that needs a Linux VM with its own disk --
# bind-mounting the project from macOS puts the files back on the shared mount,
# where the uid is not enforced and the problem stays hidden.
#
# So this copies the repository's working tree onto the VM's disk and runs it
# there. It does not catch anything runner-specific ($GITHUB_WORKSPACE, two slow
# cores, images pulled cold), and an Apple Silicon VM is arm64 while most hosted
# runners are x86.
#
# A failing page exits non-zero, but colima reports every failure as 1, so the
# page's own exit code does not survive. Read the output, not the code.
set -euo pipefail

PAGE=${1:-}
DOCCI_VERSION=${DOCCI_VERSION:-v0.12.0-mdx.1}
VM_DIR=/var/tmp/docci-linux

if [ -z "$PAGE" ]; then
  echo "usage: $0 <page.md>" >&2
  exit 2
fi
if [ ! -f "$PAGE" ]; then
  echo "$0: no such page: $PAGE" >&2
  exit 2
fi
if ! command -v colima >/dev/null 2>&1; then
  echo "$0: colima is not installed. brew install colima" >&2
  exit 2
fi

REPO=$(git rev-parse --show-toplevel)
PAGE_REL=$(cd "$(dirname "$PAGE")" && echo "$(git rev-parse --show-prefix)$(basename "$PAGE")")

if ! colima status >/dev/null 2>&1; then
  echo "== starting colima"
  colima start --cpu 4 --memory 8 --disk 60
fi

# The working tree, not HEAD: the Linux check is most wanted before you commit.
# Uncommitted work is committed inside the copy, because a page that clones its
# own project clones a commit.
# Piped rather than copied from a mount: colima shares only some host paths
# (/Users/<you> by default), so a repository anywhere else -- /private/tmp, for
# instance -- copies as an empty directory and the failure surfaces later as a
# confusing "no such file or directory".
echo "== copying the working tree onto the VM's disk"
colima ssh -- bash -lc "rm -rf '$VM_DIR' && mkdir -p '$VM_DIR'"
tar -C "$REPO" -cf - . | colima ssh -- bash -lc "tar -C '$VM_DIR' -xf -"
colima ssh -- bash -lc "
  set -euo pipefail
  case \$(df -T '$VM_DIR' | tail -1 | awk '{print \$2}') in
    ext4|xfs|btrfs) ;;
    *) echo 'refusing to run: $VM_DIR is not on the VM disk, so uid is not enforced' >&2; exit 1 ;;
  esac
  cd '$VM_DIR'
  if [ -n \"\$(git status --porcelain)\" ]; then
    git -c user.email=docci@localhost -c user.name=docci add -A
    git -c user.email=docci@localhost -c user.name=docci commit -qm 'uncommitted work under test'
    echo '   (including uncommitted changes)'
  fi
"

echo "== installing docci $DOCCI_VERSION in the VM"
colima ssh -- bash -lc "
  set -euo pipefail
  # Always install the pinned build. Reusing whatever docci the VM happens to
  # have is how you end up running an upstream binary that ignores these
  # directives, asserts nothing, and reports success.
  if true; then
    case \$(uname -m) in
      aarch64) BIN=docci_Linux_arm64.tar.gz ;;
      x86_64)  BIN=docci_Linux_x86_64.tar.gz ;;
      *) echo \"unsupported arch \$(uname -m)\" >&2; exit 1 ;;
    esac
    curl -fsSL \"https://github.com/evanorti/docci/releases/download/$DOCCI_VERSION/\$BIN\" \
      | sudo tar -xzC /usr/local/bin
    sudo chmod +x /usr/local/bin/docci
  fi
"

echo "== running $PAGE_REL"
colima ssh -- bash -lc "cd '$VM_DIR' && docci run '$PAGE_REL'"
