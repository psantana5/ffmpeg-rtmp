#!/usr/bin/env python3
"""
Automated logging migration script
Migrates standard log package calls to centralized logger
"""

import re
import sys
from pathlib import Path

def migrate_file(filepath):
    """Migrate logging calls in a single file"""
    print(f"Processing: {filepath}")
    
    with open(filepath, 'r') as f:
        content = f.read()
    
    original = content
    changes = 0
    
    # Pattern 1: log.Fatalf -> logger.Fatal(fmt.Sprintf(...))
    pattern1 = r'log\.Fatalf\("([^"]*)"([^)]*)\)'
    replacement1 = r'logger.Fatal(fmt.Sprintf("\1"\2))'
    content, count1 = re.subn(pattern1, replacement1, content)
    changes += count1
    
    # Pattern 2: log.Printf -> logger.Info(fmt.Sprintf(...))
    pattern2 = r'log\.Printf\("([^"]*)"([^)]*)\)'
    replacement2 = r'logger.Info(fmt.Sprintf("\1"\2))'
    content, count2 = re.subn(pattern2, replacement2, content)
    changes += count2
    
    # Pattern 3: log.Println("string literal") -> logger.Info("string literal")
    pattern3 = r'log\.Println\("([^"]*)"\)'
    replacement3 = r'logger.Info("\1")'
    content, count3 = re.subn(pattern3, replacement3, content)
    changes += count3
    
    # Pattern 4: log.Println(variable) -> logger.Info(variable)
    pattern4 = r'log\.Println\(([^)]+)\)'
    replacement4 = r'logger.Info(\1)'
    content, count4 = re.subn(pattern4, replacement4, content)
    changes += count4
    
    # Pattern 5: log.Fatal -> logger.Fatal
    pattern5 = r'log\.Fatal\('
    replacement5 = r'logger.Fatal('
    content, count5 = re.subn(pattern5, replacement5, content)
    changes += count5
    
    if changes > 0:
        # Check if logger is initialized
        if 'var logger' not in content and '*logging.Logger' not in content:
            print(f"  ⚠️  Warning: logger variable not found in {filepath}")
            print(f"     Add: var logger *logging.Logger")
        
        # Check if fmt is imported
        if changes > 0 and (count1 > 0 or count2 > 0):
            if '"fmt"' not in content:
                print(f"  ⚠️  Warning: fmt import needed in {filepath}")
        
        with open(filepath, 'w') as f:
            f.write(content)
        
        print(f"  ✓ Migrated {changes} log calls")
        return changes
    else:
        print(f"  No changes needed")
        return 0

def main():
    # Find all .go files (exclude test files and already-migrated files)
    root = Path(".")
    go_files = [
        f for f in root.rglob("*.go")
        if not f.name.endswith("_test.go")
        and "vendor" not in str(f)
        and "master/cmd/master/main.go" not in str(f)  # Already migrated
        and "worker/cmd/agent/main.go" not in str(f)  # Already using logger
    ]
    
    print(f"Found {len(go_files)} Go files to check")
    print("")
    
    total_changes = 0
    files_modified = 0
    
    for filepath in go_files:
        changes = migrate_file(filepath)
        if changes > 0:
            total_changes += changes
            files_modified += 1
    
    print("")
    print(f"========================================")
    print(f"Migration Summary:")
    print(f"  Files modified: {files_modified}")
    print(f"  Total log calls migrated: {total_changes}")
    print(f"========================================")
    
    if total_changes > 0:
        print("")
        print("⚠️  IMPORTANT: Manual steps required:")
        print("1. Add logger initialization where needed")
        print("2. Add 'fmt' import where Sprintf is used")
        print("3. Test compilation: go build ./...")
        print("4. Run tests: go test ./...")
        
    return 0 if files_modified == 0 else 1

if __name__ == "__main__":
    sys.exit(main())
