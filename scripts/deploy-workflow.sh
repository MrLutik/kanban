#!/bin/bash
# Deploy triage-issues.yml workflow to all repos in config
# Usage: ./deploy-workflow.sh [--dry-run]
# Uses GitHub API via gh CLI (no git clone needed)

set -euo pipefail

ORG="kiracore"
WORKFLOW_FILE="scripts/triage-issues.yml"
WORKFLOW_NAME="triage-issues.yml"
WORKFLOW_PATH=".github/workflows/$WORKFLOW_NAME"
DRY_RUN="${1:-}"

# Get repos from config (only from repositories.list section)
REPOS=$(sed -n '/^repositories:/,/^[a-z]/p' .kanban.yaml | sed -n '/list:/,/exclude:/p' | grep '^\s*-' | sed 's/.*- //' | head -30)

# Read workflow content and base64 encode
CONTENT=$(base64 -w0 < "$WORKFLOW_FILE")

echo "Deploying $WORKFLOW_NAME to $ORG repos..."
echo ""

for REPO in $REPOS; do
    echo "==> $ORG/$REPO"

    # Get default branch
    DEFAULT_BRANCH=$(unset GH_TOKEN && gh repo view "$ORG/$REPO" --json defaultBranchRef --jq '.defaultBranchRef.name' 2>/dev/null || echo "main")
    echo "    branch: $DEFAULT_BRANCH"

    if [[ "$DRY_RUN" == "--dry-run" ]]; then
        echo "    [dry-run] would create $WORKFLOW_PATH"
        continue
    fi

    # Check if file exists and get its SHA (needed for updates)
    EXISTING_SHA=""
    if unset GH_TOKEN && gh api "repos/$ORG/$REPO/contents/$WORKFLOW_PATH" > /tmp/gh_content.json 2>/dev/null; then
        EXISTING_SHA=$(jq -r '.sha' /tmp/gh_content.json)
    fi

    # Check if content is same
    if [[ -n "$EXISTING_SHA" ]]; then
        EXISTING_CONTENT=$(jq -r '.content' /tmp/gh_content.json | tr -d '\n')
        if [[ "$EXISTING_CONTENT" == "$CONTENT" ]]; then
            echo "    [skip] already up to date"
            continue
        fi
        echo "    updating existing file..."
    else
        echo "    creating new file..."
    fi

    # Build commit message
    COMMIT_MSG="Add kanban triage workflow

Automatically labels new issues:
- Maintainers: status: backlog
- Others: needs-triage
- All: repo: $REPO

🤖 Generated with kanban CLI"

    # Create or update file via API
    if [[ -n "$EXISTING_SHA" ]]; then
        # Update existing file
        if unset GH_TOKEN && gh api "repos/$ORG/$REPO/contents/$WORKFLOW_PATH" \
            --method PUT \
            -f message="$COMMIT_MSG" \
            -f content="$CONTENT" \
            -f branch="$DEFAULT_BRANCH" \
            -f sha="$EXISTING_SHA" > /dev/null 2>&1; then
            echo "    [ok] updated"
        else
            echo "    [error] update failed"
        fi
    else
        # Create new file
        if unset GH_TOKEN && gh api "repos/$ORG/$REPO/contents/$WORKFLOW_PATH" \
            --method PUT \
            -f message="$COMMIT_MSG" \
            -f content="$CONTENT" \
            -f branch="$DEFAULT_BRANCH" > /dev/null 2>&1; then
            echo "    [ok] created"
        else
            echo "    [error] create failed"
        fi
    fi
done

echo ""
echo "Done!"
