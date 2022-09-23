#!/usr/bin/env python3

import argparse
import os
import re
import shlex
import shutil
import subprocess
import sys
from configparser import ConfigParser
from datetime import datetime
from inspect import cleandoc
from itertools import chain
from os.path import basename
from pathlib import Path, PurePath

DEFAULT_ALLSEP = " "
DEFAULT_ALLFMT = "{rel}"

NON_SRC_DIRS = ["build", "dist", "__pycache__", "lib", "venv", ".tox"]


def unique(elems):
    seen = set()
    for elem in elems:
        if elem not in seen:
            yield elem
            seen.add(elem)


subprocess_run = subprocess.run


def extraargs_help(calledcmd):
    return cleandoc(
        f"""
        Additional arguments to pass on to  {calledcmd}.

        This is collected from any trailing arguments passed to `%(prog)s`.
        Use an initial `--` to separate them from regular arguments.
        """
    )


def parse_args(args=None):
    parser = argparse.ArgumentParser(description="Development helper script.")
    parser.set_defaults(parser=parser)
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Only display what would be done, don't actually do anything.",
    )
    subparsers = parser.add_subparsers(metavar="COMMAND")
    subparsers.required = True

    excparser = subparsers.add_parser(
        "exec",
        help="Run a command for each or all targets.",
        formatter_class=argparse.RawTextHelpFormatter,
        description=cleandoc(
            """Run a command according to the `format` argument for each or all targets.

        This is an advanced command that is used internally by other commands.

        For example, to install all distributions in this repository
        editable, you could use:

            scripts/eachdist.py exec "python -m pip install -e {}"

        This will run pip for all distributions which is quite slow. It gets
        a bit faster if we only invoke pip once but with all the paths
        gathered together, which can be achieved by using `--all`:

            scripts/eachdist.py exec "python -m pip install {}" --all "-e {}"

        The sortfirst option in the DEFAULT section of eachdist.ini makes
        sure that dependencies are installed before their dependents.

        Search for usages of `parse_subargs` in the source code of this script
        to see more examples.

        This command first collects target paths and then executes
        commands according to `format` and `--all`.

        Target paths are initially all Python distribution root paths
        (as determined by the existence of pyproject.toml, etc. files).
        They are then augmented according to the section of the
        `PROJECT_ROOT/eachdist.ini` config file specified by the `--mode` option.

        The following config options are available (and processed in that order):

        - `extraroots`: List of project root-relative glob expressions.
          The resulting paths will be added.
        - `sortfirst`: List of glob expressions.
          Any matching paths will be put to the front of the path list,
          in the same order they appear in this option. If more than one
          glob matches, ordering is according to the first.
        - `subglob`: List of glob expressions. Each path added so far is removed
          and replaced with the result of all glob expressions relative to it (in
          order of the glob expressions).

        After all this, any duplicate paths are removed (the first occurrence remains).
        """
        ),
    )
    excparser.set_defaults(func=execute_args)
    excparser.add_argument(
        "format",
        help=cleandoc(
            """Format string for the command to execute.

        The available replacements depend on whether `--all` is specified.
        If `--all` was specified, there is only a single replacement,
        `{}`, that is replaced with the string that is generated from
        joining all targets formatted with `--all` to a single string
        with the value of `--allsep` as separator.

        If `--all` was not specified, the following replacements are available:

        - `{}`: the absolute path to the current target in POSIX format
          (with forward slashes)
        - `{rel}`: like `{}` but relative to the project root.
        - `{raw}`: the absolute path to the current target in native format
          (thus exactly the same as `{}` on Unix but with backslashes on Windows).
        - `{rawrel}`: like `{raw}` but relative to the project root.

        The resulting string is then split according to POSIX shell rules
        (so you can use quotation marks or backslashes to handle arguments
        containing spaces).

        The first token is the name of the executable to run, the remaining
        tokens are the arguments.

        Note that a shell is *not* involved by default.
        You can add bash/sh/cmd/powershell yourself to the format if you want.

        If `--all` was specified, the resulting command is simply executed once.
        Otherwise, the command is executed for each found target. In both cases,
        the project root is the working directory.
        """
        ),
    )
    excparser.add_argument(
        "--all",
        nargs="?",
        const=DEFAULT_ALLFMT,
        metavar="ALLFORMAT",
        help=cleandoc(
            """Instead of running the command for each target, join all target
        paths together to run a single command.

        This option optionally takes a format string to apply to each path. The
        available replacements are the ones that would be available for `format`
        if `--all` was not specified.

        Default ALLFORMAT if this flag is specified: `%(const)s`.
        """
        ),
    )
    excparser.add_argument(
        "--allsep",
        help=cleandoc(
            """Separator string for the strings resulting from `--all`.
        Only valid if `--all` is specified.
        """
        ),
    )
    excparser.add_argument(
        "--allowexitcode",
        type=int,
        action="append",
        default=[0],
        help=cleandoc(
            """The given command exit code is treated as success and does not abort execution.
        Can be specified multiple times.
        """
        ),
    )
    excparser.add_argument(
        "--mode",
        "-m",
        default="DEFAULT",
        help=cleandoc(
            """Section of config file to use for target selection configuration.
        See description of exec for available options."""
        ),
    )

    instparser = subparsers.add_parser(
        "install", help="Install all distributions."
    )

    def setup_instparser(instparser):
        instparser.set_defaults(func=install_args)
        instparser.add_argument(
            "pipargs", nargs=argparse.REMAINDER, help=extraargs_help("pip")
        )

    setup_instparser(instparser)
    instparser.add_argument("--editable", "-e", action="store_true")
    instparser.add_argument("--with-test-deps", action="store_true")
    instparser.add_argument("--with-dev-deps", action="store_true")
    instparser.add_argument("--eager-upgrades", action="store_true")

    devparser = subparsers.add_parser(
        "develop",
        help="Install all distributions editable + dev dependencies.",
    )
    setup_instparser(devparser)
    devparser.set_defaults(
        editable=True,
        with_dev_deps=True,
        eager_upgrades=True,
        with_test_deps=True,
    )

    lintparser = subparsers.add_parser(
        "lint", help="Lint everything, autofixing if possible."
    )
    lintparser.add_argument("--check-only", action="store_true")
    lintparser.set_defaults(func=lint_args)

    testparser = subparsers.add_parser(
        "test",
        help="Test everything (run pytest yourself for more complex operations).",
    )
    testparser.set_defaults(func=test_args)
    testparser.add_argument(
        "pytestargs", nargs=argparse.REMAINDER, help=extraargs_help("pytest")
    )

    releaseparser = subparsers.add_parser(
        "update_versions",
        help="Updates version numbers, used by maintainers and CI",
    )
    releaseparser.set_defaults(func=release_args)
    releaseparser.add_argument("--versions", required=True)
    releaseparser.add_argument(
        "releaseargs", nargs=argparse.REMAINDER, help=extraargs_help("pytest")
    )

    fmtparser = subparsers.add_parser(
        "format", help="Formats all source code with black and isort.",
    )
    fmtparser.set_defaults(func=format_args)
    fmtparser.add_argument(
        "--path",
        required=False,
        help="Format only this path instead of entire repository",
    )

    versionparser = subparsers.add_parser(
        "version", help="Get the version for a release",
    )
    versionparser.set_defaults(func=version_args)
    versionparser.add_argument(
        "--mode",
        "-m",
        default="DEFAULT",
        help=cleandoc(
            """Section of config file to use for target selection configuration.
        See description of exec for available options."""
        ),
    )

    return parser.parse_args(args)


def find_projectroot(search_start=Path(".")):
    root = search_start.resolve()
    for root in chain((root,), root.parents):
        if any((root / marker).exists() for marker in (".git", "tox.ini")):
            return root
    return None


def find_targets_unordered(rootpath):
    for subdir in rootpath.iterdir():
        if not subdir.is_dir():
            continue
        if subdir.name.startswith(".") or subdir.name.startswith("venv"):
            continue
        if any(
            (subdir / marker).exists()
            for marker in ("pyproject.toml",)
        ):
            yield subdir
        else:
            yield from find_targets_unordered(subdir)


def getlistcfg(strval):
    return [
        val.strip()
        for line in strval.split("\n")
        for val in line.split(",")
        if val.strip()
    ]


def find_targets(mode, rootpath):
    if not rootpath:
        sys.exit("Could not find a root directory.")

    cfg = ConfigParser()
    cfg.read(str(rootpath / "eachdist.ini"))
    mcfg = cfg[mode]

    targets = list(find_targets_unordered(rootpath))
    if "extraroots" in mcfg:
        targets += [
            path
            for extraglob in getlistcfg(mcfg["extraroots"])
            for path in rootpath.glob(extraglob)
        ]
    if "sortfirst" in mcfg:
        sortfirst = getlistcfg(mcfg["sortfirst"])

        def keyfunc(path):
            path = path.relative_to(rootpath)
            for idx, pattern in enumerate(sortfirst):
                if path.match(pattern):
                    return idx
            return float("inf")

        targets.sort(key=keyfunc)
    if "ignore" in mcfg:
        ignore = getlistcfg(mcfg["ignore"])

        def filter_func(path):
            path = path.relative_to(rootpath)
            for pattern in ignore:
                if path.match(pattern):
                    return False
            return True

        filtered = filter(filter_func, targets)
        targets = list(filtered)

    subglobs = getlistcfg(mcfg.get("subglob", ""))
    if subglobs:
        targets = [
            newentry
            for newentry in (
                target / subdir
                for target in targets
                for subglob in subglobs
                # We need to special-case the dot, because glob fails to parse that with an IndexError.
                for subdir in (
                    (target,) if subglob == "." else target.glob(subglob)
                )
            )
            if ".egg-info" not in str(newentry) and newentry.exists()
        ]

    return list(unique(targets))


def runsubprocess(dry_run, params, *args, **kwargs):
    cmdstr = join_args(params)
    if dry_run:
        print(cmdstr)
        return None

    # Py < 3.6 compat.
    cwd = kwargs.get("cwd")
    if cwd and isinstance(cwd, PurePath):
        kwargs["cwd"] = str(cwd)

    check = kwargs.pop("check")  # Enforce specifying check

    print(">>>", cmdstr, file=sys.stderr, flush=True)

    # This is a workaround for subprocess.run(['python']) leaving the virtualenv on Win32.
    # The cause for this is that when running the python.exe in a virtualenv,
    # the wrapper executable launches the global python as a subprocess and the search sequence
    # for CreateProcessW which subprocess.run and Popen use is a follows
    # (https://docs.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw):
    # > 1. The directory from which the application loaded.
    # This will be the directory of the global python.exe, not the venv directory, due to the suprocess mechanism.
    # > 6. The directories that are listed in the PATH environment variable.
    # Only this would find the "correct" python.exe.

    params = list(params)
    executable = shutil.which(params[0])
    if executable:
        params[0] = executable
    try:
        return subprocess_run(params, *args, check=check, **kwargs)
    except OSError as exc:
        raise ValueError(
            "Failed executing " + repr(params) + ": " + str(exc)
        ) from exc


def execute_args(args):
    if args.allsep and not args.all:
        args.parser.error("--allsep specified but not --all.")

    if args.all and not args.allsep:
        args.allsep = DEFAULT_ALLSEP

    rootpath = find_projectroot()
    targets = find_targets(args.mode, rootpath)
    if not targets:
        sys.exit(f"Error: No targets selected (root: {rootpath})")

    def fmt_for_path(fmt, path):
        return fmt.format(
            path.as_posix(),
            rel=path.relative_to(rootpath).as_posix(),
            raw=path,
            rawrel=path.relative_to(rootpath),
        )

    def _runcmd(cmd):
        result = runsubprocess(
            args.dry_run, shlex.split(cmd), cwd=rootpath, check=False
        )
        if result is not None and result.returncode not in args.allowexitcode:
            print(
                f"'{cmd}' failed with code {result.returncode}",
                file=sys.stderr,
            )
            sys.exit(result.returncode)

    if args.all:
        allstr = args.allsep.join(
            fmt_for_path(args.all, path) for path in targets
        )
        cmd = args.format.format(allstr)
        _runcmd(cmd)
    else:
        for target in targets:
            cmd = fmt_for_path(args.format, target)
            _runcmd(cmd)


def clean_remainder_args(remainder_args):
    if remainder_args and remainder_args[0] == "--":
        del remainder_args[0]


def join_args(arglist):
    return " ".join(map(shlex.quote, arglist))


def install_args(args):
    clean_remainder_args(args.pipargs)
    if args.eager_upgrades:
        args.pipargs += ["--upgrade-strategy=eager"]

    if args.with_dev_deps:
        runsubprocess(
            args.dry_run,
            [
                "python",
                "-m",
                "pip",
                "install",
                "--upgrade",
                "pip",
                "setuptools",
                "wheel",
            ]
            + args.pipargs,
            check=True,
        )

    allfmt = "-e 'file://{}" if args.editable else "'file://{}"
    # packages should provide an extra_requires that is named
    # 'test', to denote test dependencies.
    extras = []
    if args.with_test_deps:
        extras.append("test")
    if extras:
        allfmt += f"[{','.join(extras)}]"
    # note the trailing single quote, to close the quote opened above.
    allfmt += "'"

    execute_args(
        parse_subargs(
            args,
            (
                "exec",
                "python -m pip install {} " + join_args(args.pipargs),
                "--all",
                allfmt,
            ),
        )
    )
    if args.with_dev_deps:
        rootpath = find_projectroot()
        runsubprocess(
            args.dry_run,
            [
                "python",
                "-m",
                "pip",
                "install",
                "--upgrade",
                "-r",
                str(rootpath / "dev-requirements.txt"),
            ]
            + args.pipargs,
            check=True,
        )


def parse_subargs(parentargs, args):
    subargs = parse_args(args)
    subargs.dry_run = parentargs.dry_run or subargs.dry_run
    return subargs


def lint_args(args):
    rootdir = str(find_projectroot())

    runsubprocess(
        args.dry_run,
        ("black", "--config", f"{rootdir}/pyproject.toml", ".")
        + (("--diff", "--check") if args.check_only else ()),
        cwd=rootdir,
        check=True,
    )
    runsubprocess(
        args.dry_run,
        ("isort", "--settings-path", f"{rootdir}/.isort.cfg", ".")
        + (("--diff", "--check-only") if args.check_only else ()),
        cwd=rootdir,
        check=True,
    )
    runsubprocess(args.dry_run, ("flake8", "--config", f"{rootdir}/.flake8", rootdir), check=True)
    execute_args(
        parse_subargs(
            args, ("exec", "pylint {}", "--all", "--mode", "lintroots")
        )
    )
    execute_args(
        parse_subargs(
            args,
            ("exec", "python scripts/check_for_valid_readme.py {}", "--all",),
        )
    )


def update_changelog(path, version, new_entry):
    unreleased_changes = False
    try:
        with open(path, encoding="utf-8") as changelog:
            text = changelog.read()
            if f"## [{version}]" in text:
                raise AttributeError(
                    f"{path} already contains version {version}"
                )
        with open(path, encoding="utf-8") as changelog:
            for line in changelog:
                if line.startswith("## [Unreleased]"):
                    unreleased_changes = False
                elif line.startswith("## "):
                    break
                elif len(line.strip()) > 0:
                    unreleased_changes = True

    except FileNotFoundError:
        print(f"file missing: {path}")
        return

    if unreleased_changes:
        print(f"updating: {path}")
        text = re.sub(r"## \[Unreleased\].*", new_entry, text)
        with open(path, "w", encoding="utf-8") as changelog:
            changelog.write(text)


def update_changelogs(version):
    today = datetime.now().strftime("%Y-%m-%d")
    new_entry = """## [Unreleased](https://github.com/open-telemetry/opentelemetry-python/compare/v{version}...HEAD)

## [{version}](https://github.com/open-telemetry/opentelemetry-python/releases/tag/v{version}) - {today}

""".format(
        version=version, today=today
    )
    errors = False
    try:
        update_changelog("./CHANGELOG.md", version, new_entry)
    except Exception as err:  # pylint: disable=broad-except
        print(str(err))
        errors = True

    if errors:
        sys.exit(1)


def find(name, path):
    non_src_dirs = [os.path.join(path, nsd) for nsd in NON_SRC_DIRS]

    def _is_non_src_dir(root) -> bool:
        for nsd in non_src_dirs:
            if root.startswith(nsd):
                return True
        return False

    for root, _, files in os.walk(path):
        if _is_non_src_dir(root):
            continue
        if name in files:
            return os.path.join(root, name)
    return None


def filter_packages(targets, packages):
    if not packages:
        return targets
    filtered_packages = []
    for target in targets:
        for pkg in packages:
            if str(pkg) == "all":
                continue
            if str(pkg) in str(target):
                filtered_packages.append(target)
                break
    return filtered_packages


def update_version_files(targets, version, packages):
    print("updating version.py files")
    targets = filter_packages(targets, packages)
    update_files(
        targets, "version.py", "__version__ .*", f'__version__ = "{version}"',
    )


def update_dependencies(targets, version, packages):
    print("updating dependencies")
    if "all" in packages:
        packages.extend(targets)
    for pkg in packages:
        if str(pkg) == "all":
            continue
        print(pkg)
        package_name = basename(pkg)
        print(package_name)

        update_files(
            targets,
            "pyproject.toml",
            fr"({package_name}.*)==(.*)",
            r"\1== " + version + '",',
        )


def update_files(targets, filename, search, replace):
    errors = False
    for target in targets:
        curr_file = find(filename, target)
        if curr_file is None:
            print(f"file missing: {target}/{filename}")
            continue

        with open(curr_file, encoding="utf-8") as _file:
            text = _file.read()

        if replace in text:
            print(f"{curr_file} already contains {replace}")
            continue

        with open(curr_file, "w", encoding="utf-8") as _file:
            _file.write(re.sub(search, replace, text))

    if errors:
        sys.exit(1)


def release_args(args):
    print("preparing release")

    rootpath = find_projectroot()
    targets = list(find_targets_unordered(rootpath))
    cfg = ConfigParser()
    cfg.read(str(find_projectroot() / "eachdist.ini"))
    versions = args.versions
    updated_versions = []

    excluded = cfg["exclude_release"]["packages"].split()
    targets = [target for target in targets if basename(target) not in excluded]
    for group in versions.split(","):
        mcfg = cfg[group]
        version = mcfg["version"]
        updated_versions.append(version)
        packages = None
        if "packages" in mcfg:
            packages = [pkg for pkg in mcfg["packages"].split() if pkg not in excluded]
        print(f"update {group} packages to {version}")
        update_dependencies(targets, version, packages)
        update_version_files(targets, version, packages)

    update_changelogs("-".join(updated_versions))


def test_args(args):
    clean_remainder_args(args.pytestargs)
    execute_args(
        parse_subargs(
            args,
            (
                "exec",
                "pytest {} " + join_args(args.pytestargs),
                "--mode",
                "testroots",
            ),
        )
    )


def format_args(args):
    format_dir = str(find_projectroot())
    if args.path:
        format_dir = os.path.join(format_dir, args.path)
    root_dir = str(find_projectroot())
    runsubprocess(
        args.dry_run,
        ("black", "--config", f"{root_dir}/pyproject.toml", "."),
        cwd=format_dir,
        check=True,
    )
    runsubprocess(
        args.dry_run,
        ("isort", "--settings-path", f"{root_dir}/.isort.cfg", "--profile", "black", "."),
        cwd=format_dir,
        check=True,
    )


def version_args(args):
    cfg = ConfigParser()
    cfg.read(str(find_projectroot() / "eachdist.ini"))
    print(cfg[args.mode]["version"])


def main():
    args = parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
