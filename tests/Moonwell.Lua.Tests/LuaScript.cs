using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using CSharpLua;
using Microsoft.CodeAnalysis;
using War3Net.Build;
using War3Net.Build.Extensions;

namespace Moonwell.Lua.Tests;

internal static class LuaScript
{
    private static List<string> output = new List<string>();

    public static string RepoRoot => Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", ".."));

    public static string Compile(string csproj)
    {
        var map = War3Net.Build.Map.Open(Path.Combine(RepoRoot, "tests", "Moonwell.Lua.Tests", "map"), MapFiles.All & ~MapFiles.Triggers & ~MapFiles.CustomTextTriggers);
        var outputFolder = Path.Combine(Path.GetTempPath(), "moonwell-lua", Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(outputFolder);
        var compiler = new Compiler(csproj, outputFolder, string.Empty, null, null, string.Empty, string.Empty, false, null, string.Empty)
        {
            IsExportMetadata = false,
            IsModule = false,
            IsInlineSimpleProperty = false,
            IsPreventDebugObject = true,
            IsCommentsDisabled = false,
        };

        var coreSystem = CSharpLua.CoreSystem.CoreSystemProvider.GetCoreSystemFiles()
            .Select(f => Path.Combine(AppContext.BaseDirectory, f))
            .ToList();
        coreSystem.Add(Path.Combine(RepoRoot, "src", "Moonwell.Natives", "Generated", "types.lua"));
        var moonwell = Path.Combine(RepoRoot, "src", "Moonwell", "moonwell.lua");
        if (File.Exists(moonwell))
        {
            coreSystem.Add(moonwell);
        }

        var scriptBuilder = new MapScriptBuilder(triggerData: null);
        scriptBuilder.SetDefaultOptionsForCSharpLua();
        var result = map.CompileScript(compiler, scriptBuilder, coreSystem);
        if (!result.Success)
        {
            var errors = result.Diagnostics
                .Where(d => d.Severity == DiagnosticSeverity.Error)
                .Select(d => d.ToString());
            throw new InvalidOperationException(string.Join(Environment.NewLine, errors));
        }

        return map.Script;
    }

    public static List<string> Run(string script)
    {
        output = new List<string>();
        using var lua = new NLua.Lua();
        lua.RegisterFunction("print", null, typeof(LuaScript).GetMethod(nameof(Print)));
        lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
        lua.DoString(script);
        lua.DoString("main()");
        return output;
    }

    public static void Print(string line)
    {
        output.Add(line);
    }
}
