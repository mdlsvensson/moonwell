using CSharpLua;
using Microsoft.CodeAnalysis;
using War3Net.Build;
using War3Net.Build.Extensions;

namespace Moonwell.Build
{
    /// <summary>Compiles the map project's C# to the map's Lua script.</summary>
    public static class LuaCompiler
    {
        /// <summary>Compiles the project with CSharp.lua and writes the map's script, with <c>main</c> and <c>config</c>, into the map.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map.</param>
        /// <param name="targetPath">The map's built assembly; the Moonwell libraries are next to it.</param>
        /// <param name="release">True drops <c>DEBUG</c> code and comments.</param>
        /// <returns>The script, also set on <paramref name="map"/>.</returns>
        /// <exception cref="BuildException">Common.j and Blizzard.j are missing or the C# does not compile to Lua.</exception>
        public static string Compile(MapProject project, War3Net.Build.Map map, string targetPath, bool release)
        {
            var script = Compile(project, map, targetPath, release, null, out var files);
            script = LineTable.Apply(script, files);
            map.Script = script;
            return script;
        }

        /// <summary>
        /// Compiles the project with CSharp.lua, linked with the Lua of the classes of <c>Moonwell.Objects</c> it uses, adds those
        /// classes to <c>System.init</c> and appends their fields' values, and writes the map's script into the map.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map.</param>
        /// <param name="targetPath">The map's built assembly; the Moonwell libraries are next to it.</param>
        /// <param name="release">True drops <c>DEBUG</c> code and comments.</param>
        /// <param name="declarations">What the map declares and the classes it uses.</param>
        /// <param name="ids">The object id of each declared class.</param>
        /// <param name="standard">The standard data.</param>
        /// <param name="set">The tables values are read from (<see cref="GameData.ValueSet"/>): null for the base tables.</param>
        /// <param name="trees">Whether the map references the interactions package, so that the script gets the table of its tree types.</param>
        /// <returns>The script, also set on <paramref name="map"/>.</returns>
        /// <exception cref="BuildException">Common.j and Blizzard.j are missing, the C# does not compile to Lua, a declared list names a class with no id, or an object file cannot be read.</exception>
        public static string Compile(
            MapProject project,
            War3Net.Build.Map map,
            string targetPath,
            bool release,
            DeclarationSet declarations,
            IReadOnlyDictionary<string, string> ids,
            StandardData standard,
            string? set,
            bool trees = false)
        {
            ObjectStubs.Write(project, declarations.Used, declarations.Support);
            var script = Compile(project, map, targetPath, release, ObjectLua.Definitions(declarations.Used), out var files);
            var named = new Dictionary<string, string>(declarations.StandardClassIds, StringComparer.Ordinal);
            foreach (var entry in ids)
            {
                named[entry.Key] = entry.Value;
            }

            script = ObjectLua.AddToInit(script, declarations.Used);
            // A standard class reads its type as World Editor changed it, and a type's default object any type World Editor makes.
            var mapTypes = MapTypes.Read(project.MapFolder, map.TriggerStrings, standard);
            script = ObjectLua.Values(script, declarations, named, standard, set, mapTypes, trees);
            script = LineTable.Apply(script, files);
            map.Script = script;
            return script;
        }

        // The objects' library, when given, is linked after moonwell.lua, just before the compiled code. CSharp.lua compiles the
        // marked copy of the project; files gives its marked files for the line table, which the callers add last.
        private static string Compile(MapProject project, War3Net.Build.Map map, string targetPath, bool release, string? objects, out IReadOnlyList<string> files)
        {
            var jassHelper = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments), "Warcraft III", "JassHelper");
            var commonJ = Path.Combine(jassHelper, "common.j");
            var blizzardJ = Path.Combine(jassHelper, "Blizzard.j");
            if (!File.Exists(commonJ) || !File.Exists(blizzardJ))
            {
                throw new BuildException($"common.j and Blizzard.j are not in {jassHelper}");
            }

            var luaFolder = Path.Combine(project.GeneratedFolder, "lua");
            var libraries = ScriptLibraries.Extract(targetPath, luaFolder);
            if (objects != null)
            {
                var path = Path.Combine(luaFolder, "objects.lua");
                File.WriteAllText(path, objects);
                libraries.Add(path);
            }

            // MOONWELL_LUA leaves out the generated classes of Moonwell.Objects; CSharp.lua compiles the build's stubs of them instead.
            var symbols = release ? new[] { "MOONWELL_LUA" } : new[] { "DEBUG", "MOONWELL_LUA" };
            var marked = MarkedProject.Stage(project, symbols);
            files = marked.Files;
            var compiler = new Compiler(marked.ProjectFile, project.GeneratedFolder, string.Empty, null!, null!, "*", "-define:" + string.Join(";", symbols), false, null!, string.Empty)
            {
                IsExportMetadata = false,
                IsModule = false,
                IsInlineSimpleProperty = false,
                IsPreventDebugObject = true,
                IsCommentsDisabled = release,
            };

            var builder = new MapScriptBuilder(triggerData: null!);
            builder.SetDefaultOptionsForCSharpLua();
            CompileResult result;
            try
            {
                result = map.CompileScript(compiler, builder, libraries, commonJ, blizzardJ);
            }
            catch (CompilationErrorException failure)
            {
                // CSharp.lua's own translation errors; a Roslyn error comes back as a result below.
                throw new BuildException(marked.Unstage(failure.Message));
            }
            catch (InvalidOperationException failure) when (map.Info is null || map.Environment is null)
            {
                // The fork's script builder refuses a map with no info or no environment file (W2).
                throw new BuildException(failure.Message + " Save the map in World Editor.");
            }
            if (!result.Success)
            {
                var errors = result.Diagnostics
                    .Where(diagnostic => diagnostic.Severity == DiagnosticSeverity.Error)
                    .Select(diagnostic => marked.Unstage(diagnostic.ToString()))
                    .ToList();

                // dotnet build compiled the same code, so a name missing from the stubs is the likely cause.
                if (objects != null)
                {
                    errors.Add("CSharp.lua sees only the members of Moonwell.Objects that the map's assembly references: a name used only in nameof is not one");
                }

                throw new BuildException(string.Join(Environment.NewLine, errors));
            }

            var script = ScriptLibraries.Trim(map.Script ?? throw new BuildException("CSharp.lua wrote no script"));

            return script;
        }
    }
}
