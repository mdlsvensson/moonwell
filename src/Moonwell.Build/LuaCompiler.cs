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
        /// <returns>The script, also set on <paramref name="map"/>.</returns>
        /// <exception cref="BuildException">Common.j and Blizzard.j are missing, the C# does not compile to Lua, or a declared list names a class with no id.</exception>
        public static string Compile(
            MapProject project,
            War3Net.Build.Map map,
            string targetPath,
            bool release,
            DeclarationSet declarations,
            IReadOnlyDictionary<string, string> ids,
            StandardData standard,
            string? set)
        {
            ObjectStubs.Write(project, declarations.Used, declarations.Support);
            var script = Compile(project, map, targetPath, release, ObjectLua.Definitions(declarations.Used), out var files);
            var named = new Dictionary<string, string>(declarations.StandardClassIds, StringComparer.Ordinal);
            foreach (var entry in ids)
            {
                named[entry.Key] = entry.Value;
            }

            script = ObjectLua.AddToInit(script, declarations.Used);
            script = ObjectLua.Values(script, declarations.Used, declarations.Objects, named, standard, set);
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
            LiteralCheck.Check(project.Folder, marked.Files, symbols);
            var unread = FloatLiterals.Rewrite(marked.Folder, marked.Files, Path.GetFileNameWithoutExtension(targetPath), CompiledFiles.LibrariesBeside(targetPath), symbols);
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

            // CSharp.lua's own errors come first; one only the float rewrite saw means a name it could not resolve kept its integers.
            if (unread.Count > 0)
            {
                var lines = unread.Select(marked.Unstage).Prepend("the build could not resolve these names to write whole-number floats for Lua (C18):");
                throw new BuildException(string.Join(Environment.NewLine, lines));
            }

            var script = ScriptLibraries.Trim(map.Script ?? throw new BuildException("CSharp.lua wrote no script"));

            // Without war3map.w3e War3Net writes config but no main, and the game crashes to desktop.
            if (!script.Contains("function main(", StringComparison.Ordinal))
            {
                throw new BuildException(@"the script has no main: map\war3map.w3e is missing (save the map in World Editor)");
            }

            return script;
        }
    }
}
