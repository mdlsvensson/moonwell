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
            var jassHelper = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments), "Warcraft III", "JassHelper");
            var commonJ = Path.Combine(jassHelper, "common.j");
            var blizzardJ = Path.Combine(jassHelper, "Blizzard.j");
            if (!File.Exists(commonJ) || !File.Exists(blizzardJ))
            {
                throw new BuildException($"common.j and Blizzard.j are not in {jassHelper}");
            }

            var libraries = ScriptLibraries.Extract(targetPath, Path.Combine(project.GeneratedFolder, "lua"));
            var compiler = new Compiler(project.ProjectFile, project.GeneratedFolder, string.Empty, null!, null!, "*", release ? string.Empty : "-define:DEBUG", false, null!, string.Empty)
            {
                IsExportMetadata = false,
                IsModule = false,
                IsInlineSimpleProperty = false,
                IsPreventDebugObject = true,
                IsCommentsDisabled = release,
            };

            var builder = new MapScriptBuilder(triggerData: null!);
            builder.SetDefaultOptionsForCSharpLua();
            var result = map.CompileScript(compiler, builder, libraries, commonJ, blizzardJ);
            if (!result.Success)
            {
                var errors = result.Diagnostics
                    .Where(diagnostic => diagnostic.Severity == DiagnosticSeverity.Error)
                    .Select(diagnostic => diagnostic.ToString());
                throw new BuildException(string.Join(Environment.NewLine, errors));
            }

            return map.Script ?? throw new BuildException("CSharp.lua wrote no script");
        }
    }
}
