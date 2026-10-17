using System;
using System.IO;
using System.Linq;
using Moonwell.Testing;

namespace Moonwell.Lua.Tests;

public class LuaRuntimeTests
{
    [Fact]
    public void Every_case_passes_in_Lua()
    {
        var project = Path.Combine(LuaScript.RepoRoot, "tests", "Moonwell.Testing", "Moonwell.Testing.csproj");
        var lines = LuaScript.Run(LuaScript.Compile(project));

        var failures = lines.Where(line => line.StartsWith("FAIL", StringComparison.Ordinal)).ToList();
        Assert.True(failures.Count == 0, string.Join(Environment.NewLine, failures));
        Assert.Equal("DONE " + Cases.All().Count, lines.LastOrDefault());
    }
}
