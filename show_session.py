import sys, json
lines = []
for l in sys.stdin:
    l = l.strip()
    if l:
        try:
            lines.append(json.loads(l))
        except:
            pass

for m in lines[:15]:
    role = m.get('role', '')
    if not role:
        continue
    content = m.get('content', '')
    if isinstance(content, list):
        for c in content:
            if isinstance(c, dict) and c.get('type') == 'text':
                print(f"{role}: {c['text'][:150]}")
                break
    else:
        print(f"{role}: {str(content)[:150]}")
