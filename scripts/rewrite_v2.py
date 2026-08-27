from pathlib import Path

root = Path(__file__).resolve().parents[1]
changed = []
for p in list(root.rglob("*.go")) + [root / "go.mod"]:
    t = p.read_text(encoding="utf-8")
    n = t.replace("github.com/go-kvolt/kvolt/", "github.com/go-kvolt/kvolt/v2/")
    n = n.replace('github.com/go-kvolt/kvolt"', 'github.com/go-kvolt/kvolt/v2"')
    n = n.replace("github.com/go-kvolt/kvolt'", "github.com/go-kvolt/kvolt/v2'")
    if p.name == "go.mod":
        n = n.replace("module github.com/go-kvolt/kvolt\n", "module github.com/go-kvolt/kvolt/v2\n")
        n = n.replace("module github.com/go-kvolt/kvolt\r\n", "module github.com/go-kvolt/kvolt/v2\r\n")
    if n != t:
        p.write_text(n, encoding="utf-8", newline="\n")
        changed.append(str(p.relative_to(root)))
print("\n".join(changed) if changed else "NO_CHANGES")
print("COUNT", len(changed))
