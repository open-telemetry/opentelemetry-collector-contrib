'''
This Python script runs `go mod tidy` on all modules in the repository, in a way that guarantees
convergence: no module should emit "updates to go.mod needed" errors, and running a second time
should have no effect.

This is achieved by running the command on modules in topological order, ie. running it on
dependencies before their dependent modules.

When there are groups of modules with circular dependencies between them (a "strongly connected
component", or SCC), topological order is undefined, so we apply a very naive algorithm instead.
(Example: For an SCC ABCD, this outputs the sequence "ABCD ABCD ABCD A"). This algorithm is
quadratic on the size of the SCC, so we bail out if any is larger than 10 modules.

To identify SCCs and their topological order, we use Tarjan's SCC algorithm
(see https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm).
'''

from collections import defaultdict
import os
from pathlib import Path
import re
import subprocess

print('Reading dependency graph...')

DEP_REGEX = re.compile(r'(\t|require )github.com/open-telemetry/opentelemetry-collector-contrib/(.*) v(.*)\n')

mods = []
before = defaultdict(list)
for go_mod in sorted(Path('.').rglob('go.mod')):
	mod_path = go_mod.parent
	mods.append(mod_path)
	# 'go list' is too slow as it fetches data from the proxy; parse go.mod directly.
	for line in open(go_mod):
		if (m := DEP_REGEX.fullmatch(line)):
			dep_path = Path(m.group(2))
			before[mod_path].append(dep_path)

print('Computing tidying sequence...')

mods_topo = []

next_index = 0
index = {}
min_reachable = {}
on_stack = set()
stack = []

def visit(mod):
	global next_index, mods_topo
	index[mod] = next_index
	min_reachable[mod] = next_index
	next_index += 1
	stack.append(mod)
	on_stack.add(mod)

	for mod2 in before[mod]:
		if mod2 not in index:
			visit(mod2)
		elif mod2 not in on_stack:
			continue
		min_reachable[mod] = min(min_reachable[mod], min_reachable[mod2])
	
	if min_reachable[mod] == index[mod]:
		scc = []
		while True:
			mod2 = stack.pop()
			on_stack.remove(mod2)
			scc.append(mod2)
			if mod2 == mod: break
		n = len(scc)
		if n > 1:
			if n > 10:
				print(
					'Error: large strongly connected component in dependency graph:',
		  		', '.join(str(mod) for mod in scc)
				)
				exit(1)
			# naive solution:
			for _ in range(n-1):
				mods_topo += scc
			mods_topo.append(scc[0])
		else:
			mods_topo.append(mod)

for mod in mods:
	if mod not in index:
		visit(mod)

print('Checking validity of solution...')

queue = [(mod,) for mod in mods]
while len(queue) > 0:
	path = queue.pop(0)
	i = 0
	for mod in path:
		try:
			i = mods_topo.index(mod, i)
		except ValueError:
			print('Error: Changes may not be propagated along path:', ' -> '.join(str(m) for m in path))
			exit(1)
	for dep in before[path[0]]:
		if dep in path: continue
		queue.append((dep,) + path)

print('Running `go mod tidy`...')
for mod in mods_topo:
	print(f'@ {str(mod)}')
	try:
		os.remove(mod / 'go.sum')
	except FileNotFoundError: pass
	subprocess.run(['go', 'mod', 'tidy', '-compat=1.22.0'], cwd=mod, check=True)

