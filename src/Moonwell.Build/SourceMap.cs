using War3Net.Build;
using War3Net.Build.Info;

namespace Moonwell.Build
{
    /// <summary>Opens the World Editor map the build compiles the script into.</summary>
    public static class SourceMap
    {
        /// <summary>Opens the project's map without its GUI triggers and applies the map class's settings.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="settings">The settings read from the map class; empty ones keep World Editor's values.</param>
        /// <returns>The opened map, its script language set to Lua.</returns>
        public static War3Net.Build.Map Open(MapProject project, MapSettings settings)
        {
            var map = War3Net.Build.Map.Open(project.MapFolder, MapFiles.All & ~MapFiles.Triggers & ~MapFiles.CustomTextTriggers);
            var info = map.Info ?? throw new BuildException(@"no World Editor map: map\war3map.w3i is missing");
            info.ScriptLanguage = ScriptLanguage.Lua;
            if (settings.Name.Length > 0)
            {
                info.MapName = settings.Name;
            }

            if (settings.Author.Length > 0)
            {
                info.MapAuthor = settings.Author;
            }

            if (settings.Description.Length > 0)
            {
                info.MapDescription = settings.Description;
            }

            if (settings.RecommendedPlayers.Length > 0)
            {
                info.RecommendedPlayers = settings.RecommendedPlayers;
            }

            return map;
        }
    }
}
