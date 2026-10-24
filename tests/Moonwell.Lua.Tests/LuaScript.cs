using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using CSharpLua;
using Microsoft.CodeAnalysis;
using Moonwell.Build;
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
        var copy = Rewritten(csproj, Path.Combine(outputFolder, "project"));
        var compiler = new Compiler(copy, outputFolder, string.Empty, null, null, string.Empty, string.Empty, false, null, string.Empty)
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

        // Kept next to the project copy, so a Lua error naming a line of the script can be looked up.
        File.WriteAllText(Path.Combine(outputFolder, "script.lua"), map.Script);

        return map.Script;
    }

    // A copy of the project whose whole-number floats are written as the map build writes a map's (FloatLiterals), with its
    // project references made absolute; the projects it references are compiled as they are, as a map's packages are.
    private static string Rewritten(string csproj, string folder)
    {
        var source = Path.GetDirectoryName(Path.GetFullPath(csproj));
        var files = new List<string>();
        foreach (var file in Directory.EnumerateFiles(source, "*.cs", SearchOption.AllDirectories))
        {
            var relative = Path.GetRelativePath(source, file);
            var top = relative.Split(Path.DirectorySeparatorChar)[0];
            if (top.Equals("bin", StringComparison.OrdinalIgnoreCase) || top.Equals("obj", StringComparison.OrdinalIgnoreCase))
            {
                continue;
            }

            var target = Path.Combine(folder, relative);
            Directory.CreateDirectory(Path.GetDirectoryName(target));
            File.Copy(file, target);
            files.Add(relative);
        }

        var project = Path.Combine(folder, Path.GetFileName(csproj));
        var text = Regex.Replace(
            File.ReadAllText(csproj),
            "(<ProjectReference Include=\")([^\"]+)\"",
            match => match.Groups[1].Value + Path.GetFullPath(Path.Combine(source, match.Groups[2].Value)) + "\"");
        File.WriteAllText(project, text);

        var tested = Path.GetFileNameWithoutExtension(csproj);
        var references = Directory.EnumerateFiles(AppContext.BaseDirectory, "Moonwell*.dll")
            .Where(path => Path.GetFileNameWithoutExtension(path) is var name && name != tested && name != "Moonwell.Lua.Tests" && name != "Moonwell.Build");
        var errors = FloatLiterals.Rewrite(folder, files, tested, references, Array.Empty<string>());
        if (errors.Count > 0)
        {
            throw new InvalidOperationException(string.Join(Environment.NewLine, errors));
        }

        return project;
    }

    public static List<string> Run(string script)
    {
        output = new List<string>();
        using var lua = new NLua.Lua();
        lua.RegisterFunction("print", null, typeof(LuaScript).GetMethod(nameof(Print)));
        // The game's Lua has no debug library, and CSharp.lua's core takes other paths without it (numbers, booleans and nil get
        // no methods or operators, C21): the script runs without it here too. Each native is a function that gives nil, and main
        // adds GetCameraMargin's to a number, which only the debug library's nil allowed.
        lua.DoString("debug = nil");
        lua.DoString("GetCameraMargin = function() return 0.0 end");
        lua.DoString("setmetatable(_G, { __index = function(_, name) if name ~= 'debug' then return function() return nil end end end })");
        lua.DoString(script);
        lua.DoString("main()");
        return output;
    }

    public static void Print(string line)
    {
        output.Add(line);
    }
}
