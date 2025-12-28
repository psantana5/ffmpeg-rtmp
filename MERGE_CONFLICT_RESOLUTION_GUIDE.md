# Merge Conflict Resolution Guide

## Your Situation

Your local repository (`~/Doc/proj/ffmpeg-rtmp` on staging branch) has **extensive merge conflicts** from an incomplete merge between your staging branch and the `feature/ml-regression` branch.

The conflicts show this pattern:
```
<<<<<<< HEAD
... code from staging branch ...
=======
... code from feature/ml-regression branch ...
>>>>>>> feature/ml-regression
```

## Affected Files

Based on your grep output, these files have actual merge conflicts:

### Critical Files (Many Conflicts)
1. **advisor/modeling.py** - ~18 conflicts (ML model implementation)
2. **tests/test_modeling.py** - ~30 conflicts (test suite)
3. **analyze_results.py** - ~9 conflicts (results analysis)

### Important Files (Few Conflicts)
4. **requirements.txt** - 1 conflict (dependencies)
5. **advisor/__init__.py** - 1 conflict (module exports)
6. **docs/power-prediction-model.md** - 1 conflict (documentation)
7. **README.md** - 1 conflict (documentation)

### False Positives (Not Real Conflicts)
- **setup.sh** - Lines with `====` are decorative separators, NOT conflicts
- **advisor/scoring.py** - Lines with `====` are comment decorators, NOT conflicts
- **advisor/recommender.py** - Lines with `====` are comment decorators, NOT conflicts
- **advisor/modeling.py** - Some `====` are docstring separators, NOT conflicts

## Resolution Strategy

### Option 1: Automated Script (RECOMMENDED)
Use the provided `resolve_merge_conflicts.py` script to automatically resolve conflicts by choosing the `feature/ml-regression` version (which includes the ML improvements).

### Option 2: Manual Resolution
Manually edit each file and choose which code to keep.

### Option 3: Abort and Re-merge
Abort the current merge and re-do it with better conflict resolution tools.

## Option 1: Automated Resolution (RECOMMENDED)

### Step 1: Backup Your Work
```bash
cd ~/Doc/proj/ffmpeg-rtmp
git stash  # Save any uncommitted changes
git branch backup-before-conflict-resolution  # Create backup branch
```

### Step 2: Run the Resolution Script
```bash
python3 resolve_merge_conflicts.py
```

This script will:
1. Scan for files with merge conflicts
2. For each conflict, choose the `feature/ml-regression` version (newer ML code)
3. Remove conflict markers
4. Preserve the improved ML functionality

### Step 3: Verify Changes
```bash
# Check which files were modified
git status

# Review the changes
git diff

# Test that Python files have no syntax errors
python3 -m py_compile advisor/modeling.py
python3 -m py_compile advisor/__init__.py
python3 -m py_compile analyze_results.py
```

### Step 4: Commit the Resolution
```bash
git add .
git commit -m "Resolve merge conflicts: Accept feature/ml-regression changes"
```

## Option 2: Manual Resolution

### For Each File with Conflicts:

1. **Open the file in your editor**
   ```bash
   vim advisor/modeling.py  # or code, nano, etc.
   ```

2. **Find conflict markers** (search for `<<<<<<<`)

3. **Decide which code to keep:**
   - **Keep HEAD (staging)**: Delete the `=======` to `>>>>>>>` section
   - **Keep feature/ml-regression**: Delete the `<<<<<<<` to `=======` section
   - **Keep both**: Manually merge the logic from both sections
   - **Keep neither**: Write new code

4. **Remove all conflict markers** (`<<<<<<<`, `=======`, `>>>>>>>`)

5. **Test the file**:
   ```bash
   python3 -m py_compile advisor/modeling.py
   ```

6. **Stage the resolved file**:
   ```bash
   git add advisor/modeling.py
   ```

7. **Repeat for all conflicted files**

8. **Commit the resolution**:
   ```bash
   git commit -m "Resolve merge conflicts manually"
   ```

### Conflict Resolution Decision Guide

For each conflict, consider:

**Choose feature/ml-regression (newer) if:**
- ✅ Adds ML functionality
- ✅ Includes bug fixes
- ✅ Has better documentation
- ✅ More complete implementation

**Choose HEAD (staging) if:**
- ✅ Has critical fixes not in feature branch
- ✅ More stable/tested
- ✅ Has production-specific changes

**Merge both if:**
- ✅ Both have important independent changes
- ✅ Changes are to different parts of the code

## Option 3: Abort and Re-merge

If the conflicts are too complex:

### Step 1: Abort Current Merge
```bash
cd ~/Doc/proj/ffmpeg-rtmp
git merge --abort
```

### Step 2: Use a Merge Strategy
```bash
# Option A: Accept their changes (feature/ml-regression) by default
git merge feature/ml-regression -X theirs

# Option B: Accept our changes (staging) by default  
git merge feature/ml-regression -X ours

# Option C: Use a merge tool
git mergetool
git merge feature/ml-regression
```

### Step 3: Review and Test
```bash
# Check what changed
git log --oneline -10

# Run tests
python3 -m pytest tests/
```

### Step 4: Commit
```bash
git commit
```

## Specific File Guidance

### requirements.txt
**Conflict**: Different package versions or new packages

**Resolution**: Keep both, merge into a single list
```bash
# Take both versions, deduplicate
git checkout --ours requirements.txt
git checkout --theirs requirements.txt.tmp
cat requirements.txt requirements.txt.tmp | sort | uniq > requirements.txt.merged
mv requirements.txt.merged requirements.txt
rm requirements.txt.tmp
```

### advisor/modeling.py
**Nature**: ML model implementation, likely feature/ml-regression has improvements

**Recommendation**: Accept feature/ml-regression version
```bash
git checkout --theirs advisor/modeling.py
```

### analyze_results.py
**Nature**: Results analysis with ML integration

**Recommendation**: Accept feature/ml-regression version for ML features
```bash
git checkout --theirs analyze_results.py
```

### tests/test_modeling.py
**Nature**: Tests for ML models

**Recommendation**: Accept feature/ml-regression version (updated tests for new features)
```bash
git checkout --theirs tests/test_modeling.py
```

### advisor/__init__.py
**Nature**: Module exports

**Recommendation**: Merge both - ensure all classes/functions exported
- Manually edit to include exports from both versions

### Documentation Files
**README.md, docs/power-prediction-model.md**

**Recommendation**: Merge both manually
- Keep all documentation from both versions
- Organize logically
- Remove duplicates

## After Resolution

### 1. Validate Python Syntax
```bash
# Check all Python files compile
find . -name "*.py" -not -path "./venv/*" -exec python3 -m py_compile {} \;
```

### 2. Run Tests
```bash
# Install dependencies
pip install -r requirements.txt

# Run test suite
python3 -m pytest tests/ -v
```

### 3. Verify Functionality
```bash
# Test analyze_results.py
python3 analyze_results.py --help

# Test the ML predictor
python3 -c "from advisor import MultivariatePredictor; print('Import successful')"
```

### 4. Check for Remaining Conflicts
```bash
# This should return nothing
find . -type f -not -path "./venv/*" -not -path "./.git/*" -exec grep -l "^<<<<<<< HEAD" {} \;
```

### 5. Push to Remote (Optional)
```bash
git push origin staging
```

## Common Issues

### "Both modified" After Resolution
This is normal. Stage the files with `git add`.

### Syntax Errors After Resolution
You may have accidentally kept conflict markers. Search for:
```bash
grep -r "<<<<<<< HEAD" .
grep -r "=======" .
grep -r ">>>>>>>" .
```

### Tests Failing
This may be expected if:
- Dependencies changed (run `pip install -r requirements.txt`)
- API changed between branches
- Need to update test expectations

### Import Errors
Check `advisor/__init__.py` exports match what's actually in the files.

## Getting Help

If you're stuck:

1. **Check what's conflicted**:
   ```bash
   git status | grep "both modified"
   ```

2. **See the conflict diff**:
   ```bash
   git diff --conflict=diff3 <filename>
   ```

3. **Use a visual merge tool**:
   ```bash
   git mergetool --tool=meld  # or kdiff3, vimdiff, etc.
   ```

4. **Start over from backup**:
   ```bash
   git merge --abort
   git reset --hard backup-before-conflict-resolution
   ```

## Prevention for Future

1. **Merge frequently** - Don't let branches diverge too much
2. **Small commits** - Easier to resolve conflicts in small chunks
3. **Communicate** - Coordinate with team on major changes
4. **Rebase regularly** - Keep feature branches up to date with main
5. **Use CI/CD** - Automated tests catch merge issues early

## Summary of Recommended Actions

```bash
# 1. Backup
cd ~/Doc/proj/ffmpeg-rtmp
git branch backup-before-conflict-resolution

# 2. Accept feature/ml-regression for code files (has ML improvements)
git checkout --theirs advisor/modeling.py
git checkout --theirs advisor/__init__.py
git checkout --theirs analyze_results.py
git checkout --theirs tests/test_modeling.py

# 3. Manually merge requirements.txt
# Edit requirements.txt to include all unique packages from both versions
vim requirements.txt

# 4. Manually merge documentation
# Edit to keep all docs from both versions
vim README.md
vim docs/power-prediction-model.md

# 5. Verify no conflicts remain
find . -type f -not -path "./venv/*" -not -path "./.git/*" -exec grep -l "^<<<<<<< HEAD" {} \;

# 6. Test syntax
find . -name "*.py" -not -path "./venv/*" -exec python3 -m py_compile {} \;

# 7. Commit
git add .
git commit -m "Resolve merge conflicts: Accept feature/ml-regression ML improvements"

# 8. Test
pip install -r requirements.txt
python3 -m pytest tests/

# 9. Push
git push origin staging
```
