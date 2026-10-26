using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using Moonwell.Build;
using War3Net.Build;

namespace Moonwell.Lua.Tests;

internal static class LuaScript
{
    private static List<string> output = new List<string>();

    public static string RepoRoot => Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", ".."));

    /// <summary>
    /// Compiles a project as a map build compiles a map without Moonwell.Objects (LuaCompiler.Compile, as MapBuild calls it): a
    /// copy of the project with the test map is a map project, whose files are marked for the line table (MarkedProject,
    /// LineMarks) and compiled by CSharp.lua with DEBUG and MOONWELL_LUA for a development build, MOONWELL_LUA alone and no
    /// comments for a release build; the script is linked with CSharp.lua's core and the Lua that Moonwell.Natives.dll and
    /// Moonwell.dll carry (ScriptLibraries), trimmed, and given its line table. What a map build adds besides (the map's entry,
    /// its objects, the placed things) is no part of the cases.
    /// </summary>
    public static string Compile(string csproj, bool release)
    {
        var outputFolder = Path.Combine(Path.GetTempPath(), "moonwell-lua", Guid.NewGuid().ToString("N"));
        var folder = Path.Combine(outputFolder, "project");
        Rewritten(csproj, folder);
        var mapFolder = Path.Combine(folder, "map");
        Directory.CreateDirectory(mapFolder);
        foreach (var file in Directory.EnumerateFiles(Path.Combine(RepoRoot, "tests", "Moonwell.Lua.Tests", "map")))
        {
            File.Copy(file, Path.Combine(mapFolder, Path.GetFileName(file)));
        }

        var project = MapProject.Find(folder);
        var map = War3Net.Build.Map.Open(project.MapFolder, MapFiles.All & ~MapFiles.Triggers & ~MapFiles.CustomTextTriggers);

        // The libraries the script is linked with are read from beside the project's assembly, as a map build reads them from
        // beside the map's: here the ones this test was built with.
        var assembly = Path.Combine(AppContext.BaseDirectory, Path.GetFileNameWithoutExtension(csproj) + ".dll");
        var script = LuaCompiler.Compile(project, map, assembly, release);

        // Kept next to the project copy, so a Lua error naming a line of the script can be looked up.
        File.WriteAllText(Path.Combine(outputFolder, "script.lua"), script);

        return script;
    }

    // A copy of the project with its project references made absolute; the projects it references are compiled as they are,
    // as a map's packages are.
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

        // The cases' logged errors are compared with .NET's, which have no C# location: the line table is emptied, so an error's
        // location is still looked for and none is found. ErrorLinesTests (Moonwell.Build.Tests) checks the locations in a built map.
        lua.DoString("MoonwellLines = { files = {}, map = \"\" }");
        lua.DoString("main()");
        return output;
    }

    public static void Print(string line)
    {
        output.Add(line);
    }
}
