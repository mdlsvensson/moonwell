using System;
using System.IO;
using System.Linq;
using Moonwell.Testing;

namespace Moonwell.Lua.Tests;

/// <summary>
/// Every case compiled as a map's development build and as its release build, each run in Lua: a development build keeps the
/// runtime's Dev.Check calls, the development error path and the cases' #if DEBUG checks, which a release build leaves out.
/// </summary>
public class LuaRuntimeTests
{
    [Fact]
    public void Every_case_passes_in_Lua_in_a_development_build()
    {
        EveryCasePasses(release: false);
    }

    [Fact]
    public void Every_case_passes_in_Lua_in_a_release_build()
    {
        EveryCasePasses(release: true);
    }

    private static void EveryCasePasses(bool release)
    {
        var project = Path.Combine(LuaScript.RepoRoot, "tests", "Moonwell.Testing", "Moonwell.Testing.csproj");
        var lines = LuaScript.Run(LuaScript.Compile(project, release));

        var failures = lines.Where(line => line.StartsWith("FAIL", StringComparison.Ordinal)).ToList();
        Assert.True(failures.Count == 0, string.Join(Environment.NewLine, failures));
        Assert.Equal("DONE " + Cases.All().Count, lines.LastOrDefault());
    }
}
