using War3Net.Build;
using War3Net.Build.Info;

namespace Moonwell.Build
{
    /// <summary>Opens the World Editor map the build compiles the script into.</summary>
    public static class SourceMap
    {
        // Only what War3Net's script builder reads and the info file; the other files are copied as they are.
        private const MapFiles ScriptFiles = MapFiles.Info | MapFiles.Units | MapFiles.UnitObjectData | MapFiles.Doodads
            | MapFiles.Regions | MapFiles.Cameras | MapFiles.Sounds | MapFiles.Environment | MapFiles.TriggerStrings;

        /// <summary>Opens the project's map without its GUI triggers and applies the map class's settings, its game data option among them.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="settings">The settings read from the map class; empty ones keep World Editor's values.</param>
        /// <param name="log">The build log; a game data option the map class changes is noted in it.</param>
        /// <returns>The opened map, its script language set to Lua.</returns>
        public static War3Net.Build.Map Open(MapProject project, MapSettings settings, TextWriter log)
        {
            War3Net.Build.Map map;
            try
            {
                map = War3Net.Build.Map.Open(project.MapFolder, ScriptFiles);
            }
            catch (Exception failure) when (failure is not BuildException)
            {
                throw new BuildException($@"cannot read the World Editor map in map\: {failure.Message}");
            }

            var info = map.Info ?? throw new BuildException(@"no World Editor map: map\war3map.w3i is missing");
            info.ScriptLanguage = ScriptLanguage.Lua;
            var set = info.GameDataSet;
            var version = info.GameDataVersion;
            var data = GameData.Of(settings.DataSet, settings.DataVersion);
            data.Apply(info);
            if (info.GameDataSet != set || info.GameDataVersion != version)
            {
                log.WriteLine($"note: the map's game data option {GameData.Describe(set, version)} is replaced by {data}");
            }

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
