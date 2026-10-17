using War3Net.Build.Extensions;

namespace Moonwell.Build
{
    /// <summary>Writes the staged map folder: the files the archive holds.</summary>
    public static class Stage
    {
        private static readonly string[] IgnoredMapFiles =
        {
            "war3map.wtg",
            "war3map.wct",
            "war3map.lua",
            "war3map.j",
            "war3map.w3i",
        };

        /// <summary>Deletes the staged folder and writes it anew from the map's files, the info file, the script and the assets.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="map">The opened source map, with the script set.</param>
        /// <param name="log">Where notes for the modder go.</param>
        /// <exception cref="BuildException">The map has no script.</exception>
        public static void Write(MapProject project, War3Net.Build.Map map, TextWriter log)
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
