using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
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

    /// <summary>
    /// Runs a script's main in a Lua made as like the game's as NLua allows, and returns what it printed.
    /// <para>
    /// The game's Lua has no collectgarbage, dofile, loadfile, debug, io, package or require, and its os only clock, date,
    /// difftime and time (game-facts, Lua runtime): they are taken out before the script loads, so CoreSystem's own copies
    /// of them are nil too, as in the game. The cases that read Lua's memory on purpose (MissCases) call the host's
    /// collectgarbage under the name LuaHostCollectGarbage, which only this host has. CoreSystem takes other paths without
    /// the debug library (numbers, booleans and nil get no methods or operators, C21).
    /// </para>
    /// <para>
    /// Each native and global of common.j and Blizzard.j (the generated Moonwell.Natives) is a function that gives nil, but
    /// GetCameraMargin gives 0, since main adds it to a number. Reading any other global the script has not set gives nil
    /// while the script loads, as in the game, and then, while main runs the cases, an error naming it, so a misspelt or
    /// missing native fails here instead of reading nil; a global the game leaves out still reads nil, as there.
    /// </para>
    /// <para>
    /// What this host cannot show: NLua runs Lua 5.4 with 64-bit integers and doubles, where the game runs 5.3 with 32-bit
    /// integers and single floats. No case here sees an integer wrap at 2^31, a float rounded to single precision or printed
    /// as the game prints it, the game's math.ceil(0.0) of 1, or a call to what only 5.4 has (warn, coroutine.close).
    /// </para>
    /// </summary>
    public static List<string> Run(string script)
    {
        output = new List<string>();
        using var lua = new NLua.Lua();
        lua.RegisterFunction("print", null, typeof(LuaScript).GetMethod(nameof(Print)));
        lua.DoString("""
            LuaHostCollectGarbage = collectgarbage
            collectgarbage = nil
            dofile = nil
            loadfile = nil
            debug = nil
            io = nil
            package = nil
            require = nil
            os = { clock = os.clock, date = os.date, difftime = os.difftime, time = os.time }
            GetCameraMargin = function() return 0.0 end
            """);
        lua.DoString(Globals(strict: false));
        lua.DoString(script);

        // The cases' logged errors are compared with .NET's, which have no C# location: the line table is emptied, so an error's
        // location is still looked for and none is found. ErrorLinesTests (Moonwell.Build.Tests) checks the locations in a built map.
        lua.DoString("MoonwellLines = { files = {}, map = \"\" }");
        lua.DoString(Globals(strict: true));
        lua.DoString("main()");
        return output;
    }

    public static void Print(string line)
    {
        output.Add(line);
    }

    // The globals' metatable: a native or global of common.j and Blizzard.j reads as a function that gives nil, and a global the
    // game leaves out reads nil; any other name reads nil, or with strict raises an error naming it.
    private static string Globals(bool strict)
    {
        var lua = new StringBuilder();
        lua.Append("local natives = {");
        foreach (var name in NativeNames.Value)
        {
            lua.Append(name).Append(" = true, ");
        }

        lua.Append("}\n");
        lua.Append("local absent = { collectgarbage = true, dofile = true, loadfile = true, debug = true, io = true, package = true, require = true }\n");
        lua.Append("local strict = ").Append(strict ? "true" : "false").Append('\n');
        lua.Append("local function native() return nil end\n");
        lua.Append("setmetatable(_G, { __index = function(_, name)\n");
        lua.Append("  if natives[name] then return native end\n");
        lua.Append("  if absent[name] or not strict then return nil end\n");
        lua.Append("  error(\"the game has no global '\" .. tostring(name) .. \"'\", 2)\n");
        lua.Append("end })\n");
        return lua.ToString();
    }

    // The Lua names of the natives and globals of common.j and Blizzard.j, as the generated Natives class writes them; its
    // constants are C# constants, whose values CSharp.lua writes in place of their names, but main reads some by name.
    private static readonly Lazy<IReadOnlyList<string>> NativeNames = new Lazy<IReadOnlyList<string>>(() =>
    {
        var natives = File.ReadAllText(Path.Combine(RepoRoot, "src", "Moonwell.Natives", "Generated", "Natives.g.cs"));
        return Regex.Matches(natives, "@CSharpLua\\.(?:Template|Get|Set) = \"([A-Za-z_][A-Za-z0-9_]*)|public const \\w+ ([A-Za-z_][A-Za-z0-9_]*) =")
            .Select(match => match.Groups[1].Success ? match.Groups[1].Value : match.Groups[2].Value)
            .Distinct(StringComparer.Ordinal)
            .ToList();
    });
}
