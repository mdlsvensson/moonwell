using War3Net.Build.Extensions;

namespace Moonwell.Build
{
    /// <summary>Writes the staged map folder: the files the archive holds.</summary>
    public static class Stage
    {
        // The object files are written by ObjectFiles, or copied as they are when the map does not use Moonwell.Objects.
        private static readonly string[] ObjectFileNames =
        {
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

        // The script and info files are regenerated. The trigger files stay: World Editor needs them to open the map, so they are copied unparsed.
        private static readonly string[] IgnoredMapFiles = new[] { "war3map.lua", "war3map.j", "war3map.w3i" }.Concat(ObjectFileNames).ToArray();

        /// <summary>Stages a map project that does not use <c>Moonwell.Objects</c>: the source map's object files are copied as they are, with no note about them.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map, with the script set.</param>
        /// <param name="log">Where notes for the modder go.</param>
        /// <exception cref="BuildException">The map has no script.</exception>
        public static void WriteWithoutObjects(MapProject project, War3Net.Build.Map map, TextWriter log)
        {
            Write(
                project,
                map,
                staged =>
                {
                    foreach (var file in ObjectFileNames)
                    {
                        if (File.Exists(Path.Combine(project.MapFolder, file)))
                        {
                            Copy(project.MapFolder, file, staged);
                        }
                    }
                },
                log);
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
            Write(project, map, staged => ObjectFiles.Write(project, objects, ids, standardClassIds, standard, staged, log), log);
        }

        private static void Write(MapProject project, War3Net.Build.Map map, Action<string> writeObjectFiles, TextWriter log)
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
            writeObjectFiles(project.StagedFolder);

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
