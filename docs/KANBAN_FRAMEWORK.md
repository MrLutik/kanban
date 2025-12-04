# Kanban Framework for Issue-Based Project Management

A lightweight Kanban system for open source projects using native issue trackers (GitHub, GitLab, etc.)

---

## Core Principles

1. **Visualize your workflow** - Make work visible through labels/tags
2. **Limit work in progress (WIP)** - Focus on finishing, not starting
3. **Manage flow** - Identify and resolve bottlenecks
4. **Make policies explicit** - Everyone knows the rules
5. **Continuous improvement** - Regularly reflect and adjust

---

## Label Structure

### Status Labels (Your Kanban Columns)

Create these labels in your issue tracker:

- **`status: backlog`** - Prioritized but not started
- **`status: ready`** - Ready to be worked on (all dependencies clear)
- **`status: in-progress`** - Actively being worked on
- **`status: review`** - Waiting for code review/feedback
- **`status: testing`** - Being tested/validated
- **`status: done`** - Completed and merged/deployed

**Color coding suggestion:**
- Backlog: Gray (#d4d4d4)
- Ready: Blue (#0075ca)
- In Progress: Yellow (#fbca04)
- Review: Orange (#d93f0b)
- Testing: Purple (#a371f7)
- Done: Green (#0e8a16)

### Priority Labels

- **`priority: critical`** - Drop everything
- **`priority: high`** - Next up
- **`priority: medium`** - Normal priority
- **`priority: low`** - When time permits

### Type Labels

- **`type: bug`** - Something is broken
- **`type: feature`** - New functionality
- **`type: improvement`** - Enhancement to existing feature
- **`type: docs`** - Documentation work
- **`type: refactor`** - Code quality improvement
- **`type: chore`** - Maintenance tasks

### Size Labels (Optional but Recommended)

- **`size: XS`** - < 1 hour
- **`size: S`** - 1-4 hours
- **`size: M`** - 1-2 days
- **`size: L`** - 3-5 days
- **`size: XL`** - > 1 week (consider breaking down)

---

## Workflow Rules

### Column Definitions and Entry Criteria

#### Backlog
- **What it is:** All identified work, roughly prioritized
- **Entry criteria:** Issue created with description
- **Exit criteria:** Moved to Ready when prioritized for immediate work
- **WIP limit:** None

#### Ready
- **What it is:** Next items to be worked on
- **Entry criteria:**
  - Requirements are clear
  - Dependencies are resolved
  - Assigned to someone OR available for anyone to pick up
- **Exit criteria:** Someone starts working on it
- **WIP limit:** 5-10 issues (adjust based on team size)

#### In Progress
- **What it is:** Active development
- **Entry criteria:**
  - Assigned to someone
  - Work has actually started (branch created, first commit made)
- **Exit criteria:** Work complete and PR/MR created
- **WIP limit:** 1-2 per person (strict!)

#### Review
- **What it is:** Awaiting code review or approval
- **Entry criteria:**
  - PR/MR created and linked to issue
  - All tests passing
  - Ready for reviewer
- **Exit criteria:** Approved by required reviewers
- **WIP limit:** Monitor for bottlenecks (if >10, reviews are too slow)

#### Testing
- **What it is:** Being validated/tested
- **Entry criteria:**
  - Code merged to testing branch/environment
  - Test cases identified
- **Exit criteria:** All tests pass, acceptance criteria met
- **WIP limit:** 5-8 issues

#### Done
- **What it is:** Completed and deployed/merged to main
- **Entry criteria:**
  - Merged to main branch
  - Deployed (if applicable)
  - Acceptance criteria met
- **Exit criteria:** Issue closed
- **WIP limit:** None (close issues regularly)

---

## Daily Workflow

### Morning Routine
1. Check your "In Progress" items - aim to finish before starting new work
2. If blocked, move issue back to Ready and add `blocked` label with comment
3. Pull next item from Ready column when capacity available

### Moving Issues
**Always add a comment when changing status** explaining:
- What was done
- What's next
- Any blockers or concerns

### Weekly Cleanup
- Close all issues in Done column
- Groom Backlog (prioritize, close stale issues)
- Check WIP limits and bottlenecks
- Review metrics (see below)

---

## WIP Limits (Adjust to Your Team)

**Why WIP limits matter:** They force you to finish work before starting new work, which actually increases throughput.

### Suggested Starting Limits
- **Ready:** 2x your team size
- **In Progress:** 1-2 per person (start with 1!)
- **Review:** No hard limit, but monitor for bottlenecks
- **Testing:** 1-2x your team size

### What to do when you hit a WIP limit
1. **Don't start new work** - help finish existing work instead
2. **Swarm on blockers** - pair up to unblock issues
3. **Review PRs** - help move items through Review
4. **Write tests** - help move items through Testing

---

## Best Practices

### For Issue Creation
```markdown
## Description
Clear description of the problem or feature

## Acceptance Criteria
- [ ] Specific, testable criteria
- [ ] Another criteria
- [ ] Final criteria

## Dependencies
- Depends on #123
- Blocks #456

## Notes
Any additional context
```

### For Moving Issues
- **Backlog → Ready:** Ensure issue is well-defined, add size label
- **Ready → In Progress:** Assign to yourself, create branch
- **In Progress → Review:** Create PR, link issue, ensure tests pass
- **Review → Testing:** Merge to testing branch
- **Testing → Done:** Merge to main, verify in production

### Handling Blocked Issues
1. Add `blocked` label
2. Add comment explaining the blocker
3. Link to blocking issue if internal
4. Move back to Ready or In Progress (not Review)
5. Pick up something else

---

## Metrics to Track

### Lead Time
Time from issue creation to Done (measures total delivery time)

### Cycle Time
Time from In Progress to Done (measures active work time)

### Throughput
Number of issues completed per week

### WIP
Current number of issues in each column

### Review these weekly and ask:**
- Are we finishing work or just starting it?
- Where are bottlenecks forming?
- Are our WIP limits appropriate?
- What's our average cycle time?

---

## GitHub/GitLab Specific Tips

### GitHub Projects
- Create a Projects board with your status labels as columns
- Enable auto-assignment when issues move to "In Progress"
- Use automation: "When issue is closed, move to Done"

### GitLab Boards
- Use issue board with label-based lists
- Create board scopes for different workstreams
- Use quick actions in comments: `/label ~status:in-progress`

### Issue Templates
Create templates for common issue types to ensure consistency

### Automation Ideas
```yaml
# Example: Auto-label PRs based on branch name
- if branch starts with "feature/": add type:feature
- if branch starts with "fix/": add type:bug
- if PR created: add status:review
- if PR merged: add status:done
```

---

## Common Pitfalls to Avoid

1. **Too many WIP items** - You're starting work but not finishing it
2. **Skipping the Ready column** - Items aren't properly prepared
3. **Not closing Done items** - Your board gets cluttered
4. **No size estimation** - Can't gauge capacity or planning
5. **Review bottlenecks** - Need faster code review process
6. **Ignoring WIP limits** - The system breaks down

---

## Starting Your Kanban System

### Week 1: Setup
1. Create all status labels in your issue tracker
2. Create a project board with status columns
3. Move all existing issues to Backlog
4. Identify 5-10 issues for Ready column

### Week 2: Start Flowing
1. Set initial WIP limits (be conservative)
2. Team members pull from Ready when they have capacity
3. Practice moving issues and adding comments
4. Have a mid-week check-in

### Week 3: First Retrospective
1. Review your flow
2. Identify bottlenecks
3. Adjust WIP limits if needed
4. Refine your process

### Monthly: Continuous Improvement
- Review metrics
- Adjust labels/columns as needed
- Update workflow rules
- Celebrate improvements

---

## Quick Reference Card

**Daily:**
- [ ] Finish before starting
- [ ] Comment when moving issues
- [ ] Respect WIP limits
- [ ] Help unblock others

**Weekly:**
- [ ] Close Done issues
- [ ] Groom Backlog
- [ ] Review metrics
- [ ] Retrospect and adjust

**Per Issue:**
- [ ] Clear description
- [ ] Acceptance criteria
- [ ] Size estimate
- [ ] Status label
- [ ] Priority label
- [ ] Type label

---

## Example: Issue Lifecycle

```
1. Developer creates issue #123: "Add user export feature"
   Labels: type:feature, priority:high
   
2. Team lead refines issue, adds acceptance criteria
   Labels: type:feature, priority:high, status:backlog, size:M
   
3. Issue prioritized for next sprint
   Labels: ..., status:ready
   
4. Developer picks up issue
   Labels: ..., status:in-progress
   Assigned: @developer
   
5. Developer creates PR, links issue
   Labels: ..., status:review
   
6. After review, merged to testing
   Labels: ..., status:testing
   
7. QA validates, merges to main
   Labels: ..., status:done
   
8. Issue closed
   Status: Closed
```

---

## Customization

This framework is a starting point. Adapt it to your needs:

- **Remove columns** if your workflow is simpler (e.g., no separate Testing)
- **Add columns** if needed (e.g., "Design" for UI work)
- **Adjust WIP limits** based on your team's velocity
- **Create sub-labels** for more granular tracking (e.g., `type:bug:critical`)
- **Add epics/milestones** for larger features

The best Kanban system is one your team actually uses. Start simple, then evolve.

---

**Remember:** Kanban is about making work visible and flowing. Keep it simple, inspect regularly, and adapt continuously.
