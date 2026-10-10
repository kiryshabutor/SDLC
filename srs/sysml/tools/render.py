"""Рендер SysML v2 моделей в PNG через SysML v2 Pilot Implementation (Jupyter-ядро).

Использование:
  python render.py OUT_DIR model1.sysml [model2.sysml ...] -- 'имя|Элемент|опции %viz|wire' ...

Каждая спецификация — имя PNG-файла, квалифицированное имя элемента, опции %viz
через «|». Флаг «wire» включает раскладку макетов: дети элементов типа Screen,
Column, Form и ListView располагаются сверху вниз, остальных — слева направо
(в сгенерированный PlantUML добавляются невидимые связи, модель не меняется).
"""
import re
import subprocess
import sys

import cairosvg
import nbformat
from nbclient import NotebookClient

KERNEL_CP = "/opt/conda/share/jupyter/kernels/sysml/*"
VERTICAL = re.compile(r": (Screen|Column|Form|ListView)\"")
NODE = re.compile(r'^rec \w+ "(.*)" as (E\d+)')


def wireframe(puml: str) -> str:
    """Добавляет невидимые связи между соседними элементами для раскладки макета."""
    stack, children, links = [("root", False)], {"root": []}, []
    for line in puml.splitlines():
        m = NODE.match(line)
        if m:
            node = m.group(2)
            children[stack[-1][0]].append(node)
            children[node] = []
            if line.rstrip().endswith("{"):
                stack.append((node, bool(VERTICAL.search(line))))
        elif line.strip() == "}" and len(stack) > 1:
            node, vertical = stack.pop()
            kids = children[node]
            if not vertical:  # при left to right direction ребро ставит соседей в строку
                links += list(zip(kids, kids[1:]))
    def leaf(n):  # связи между контейнерами ведём через их первые листовые элементы
        return leaf(children[n][0]) if children[n] else n

    links = [f"{leaf(a)} -[hidden]- {leaf(b)}" for a, b in links]
    puml = re.sub(r'"<\w+> ', '"', puml)  # короткие имена в подписях не нужны
    puml = puml.replace("@startuml", "@startuml\nleft to right direction", 1)
    return puml.replace("@enduml", "\n".join(links) + "\n@enduml")


def main():
    args = sys.argv[1:]
    out, sep = args[0], args.index("--")
    files, specs = args[1:sep], [s.split("|") for s in args[sep + 1:]]
    nb = nbformat.v4.new_notebook()
    for f in files:
        nb.cells.append(nbformat.v4.new_code_cell(open(f, encoding="utf-8").read()))
    for name, elem, *opts in specs:
        opts = [o for o in opts if o != "wire"] + (["--style=PUMLCODE"] if "wire" in opts else [])
        nb.cells.append(nbformat.v4.new_code_cell("%viz " + " ".join(opts + [elem])))
    NotebookClient(nb, kernel_name="sysml", timeout=600, allow_errors=True).execute()

    ok = True
    for f, cell in zip(files, nb.cells):
        for o in cell.outputs:
            text = o.get("text") or o.get("data", {}).get("text/plain", "")
            print(f"{f}: {text.strip()}")
            ok &= "ERROR" not in text
    for (name, elem, *opts), cell in zip(specs, nb.cells[len(files):]):
        data = {k: v for o in cell.outputs for k, v in o.get("data", {}).items()}
        if "wire" in opts and data.get("text/plain", "").startswith("@startuml"):
            puml = wireframe(data["text/plain"])
            svg = subprocess.run(["java", "-cp", KERNEL_CP, "net.sourceforge.plantuml.Run", "-tsvg", "-pipe", "-charset", "UTF-8"],
                                 input=puml.encode(), capture_output=True, check=True).stdout.decode()
        elif "image/svg+xml" in data:
            svg = data["image/svg+xml"]
        else:
            print(f"{name}: НЕ ОТРИСОВАНО: {data or cell.outputs}")
            ok = False
            continue
        cairosvg.svg2png(bytestring=svg.encode(), write_to=f"{out}/{name}.png", scale=1.5, background_color="white")
        print(f"{name}: OK")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
