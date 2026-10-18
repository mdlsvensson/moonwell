using War3Net.Build.Extensions;

namespace Moonwell.Build
{
    /// <summary>Writes the staged map folder: the files the archive holds.</summary>
    public static class Stage
    {
        // The script and info files are regenerated, and the object files written by ObjectFiles. The trigger files stay: World Editor needs them to open the map, so they are copied unparsed.
        private static readonly string[] IgnoredMapFiles =
        {
            "war3map.lua",
            "war3map.j",
            "war3map.w3i",
            "war3map.w3u",
            "war3map.w3t",
            "war3map.w3b",
            "war3map.w3d",
            "war3map.w3a",
            "war3map.w3h",
            "war3map.w3q",
            "war3mapSkin.w3u",
            "war3mapSkin.w3t",
            "war3mapSkin.w3b",
            "war3mapSkin.w3d",
            "war3mapSkin.w3a",
            "war3mapSkin.w3h",
            "war3mapSkin.w3q",
        };

        /// <summary>Stages a map that declares no objects: its own object files are kept as they are.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map, with the script set.</param>
        /// <param name="log">Where notes for the modder go.</param>
        /// <exception cref="BuildException">The map has no script.</exception>
        public static void Write(MapProject project, War3Net.Build.Map map, TextWriter log)
        {
            Write(project, map, Array.Empty<DeclaredObject>(), new Dictionary<string, string>(), new Dictionary<string, string>(), StandardData.Empty, log);
        }

        /// <summary>Deletes the staged folder and writes it anew from the map's files, the info file, the script, the object files and the assets.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map, with the script set.</param>
        /// <param name="objects">The declared objects, for <see cref="ObjectFiles.Write"/>.</param>
        /// <param name="ids">The object id of each declared class and carrier.</param>
        /// <param name="standardClassIds">The standard id of each standard class a declared list or reference names.</param>
        /// <param name="standard">The standard data.</param>
        /// <param name="log">Where notes and warnings for the modder go.</param>
        /// <exception cref="BuildException">The map has no script, or a declared list names a class that has no object.</exception>
        public static void Write(
            MapProject project,
            War3Net.Build.Map map,
            IReadOnlyList<DeclaredObject> objects,
            IReadOnlyDictionary<string, string> ids,
            IReadOnlyDictionary<string, string> standardClassIds,
            StandardData standard,
            TextWriter log)
        {
            var script = map.Script ?? throw new BuildException("the map has no script to stage");

            if (Directory.Exists(project.StagedFolder))
            {
                Directory.Delete(project.StagedFolder, true);
            }

            Directory.CreateDirectory(project.StagedFolder);

            foreach (var file in RelativeFiles(project.MapFolder))
            {
                if (!IgnoredMapFiles.Contains(file, StringComparer.OrdinalIgnoreCase))
                {
                    Copy(project.MapFolder, file, project.StagedFolder);
                }
            }

            using (var info = map.GetInfoFile() ?? throw new BuildException(@"no World Editor map: map\war3map.w3i is missing"))
            using (var infoStream = info.MpqStream)
            using (var staged = File.Create(Path.Combine(project.StagedFolder, "war3map.w3i")))
            {
                infoStream.CopyTo(staged);
            }

            File.WriteAllText(Path.Combine(project.StagedFolder, "war3map.lua"), script);
            ObjectFiles.Write(project, objects, ids, standardClassIds, standard, project.StagedFolder, log);

            if (Directory.Exists(project.AssetsFolder))
            {
                foreach (var file in RelativeFiles(project.AssetsFolder))
                {
                    if (File.Exists(Path.Combine(project.StagedFolder, file)))
                    {
                        log.WriteLine($@"note: assets\{file} replaces the map's file");
                    }

                    Copy(project.AssetsFolder, file, project.StagedFolder);
                }
            }
        }

        private static IEnumerable<string> RelativeFiles(string folder)
        {
            return Directory.GetFiles(folder, "*", SearchOption.AllDirectories)
                .Select(file => Path.GetRelativePath(folder, file))
                .OrderBy(file => file, StringComparer.Ordinal);
        }

        private static void Copy(string fromFolder, string relative, string toFolder)
        {
            var target = Path.Combine(toFolder, relative);
            Directory.CreateDirectory(Path.GetDirectoryName(target)!);
            File.Copy(Path.Combine(fromFolder, relative), target, true);
        }
    }
}
