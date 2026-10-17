namespace Moonwell.Build
{
    /// <summary>A map project: one C# project and the World Editor map folder beside it.</summary>
    public sealed class MapProject
    {
        private MapProject(string folder, string projectFile)
        {
            Folder = folder;
            ProjectFile = projectFile;
            Name = Path.GetFileNameWithoutExtension(projectFile);
            MapFolder = Path.Combine(folder, "map");
            AssetsFolder = Path.Combine(folder, "assets");
            BuildFolder = Path.Combine(folder, "build");
            StagedFolder = Path.Combine(BuildFolder, Name);
            ArchivePath = Path.Combine(BuildFolder, Name + ".w3x");
            GeneratedFolder = Path.Combine(BuildFolder, "generated");
        }

        /// <summary>The full path of the project folder, without a trailing separator.</summary>
        public string Folder { get; }

        /// <summary>The full path of the C# project file.</summary>
        public string ProjectFile { get; }

        /// <summary>The project file name without <c>.csproj</c>; the map's name.</summary>
        public string Name { get; }

        /// <summary>The World Editor map folder, <c>map</c> under the project folder.</summary>
        public string MapFolder { get; }

        /// <summary>The folder of files that override or add to the map, <c>assets</c> under the project folder.</summary>
        public string AssetsFolder { get; }

        /// <summary>The folder the build writes into, <c>build</c> under the project folder.</summary>
        public string BuildFolder { get; }

        /// <summary>The folder holding the staged map, <c>build\&lt;Name&gt;</c>.</summary>
        public string StagedFolder { get; }

        /// <summary>The map archive, <c>build\&lt;Name&gt;.w3x</c>.</summary>
        public string ArchivePath { get; }

        /// <summary>The folder for generated C#, <c>build\generated</c>.</summary>
        public string GeneratedFolder { get; }

        /// <summary>Finds the map project at a folder or at a <c>.csproj</c> file.</summary>
        /// <param name="path">A project folder, or the path of its <c>.csproj</c> file.</param>
        /// <returns>The map project.</returns>
        /// <exception cref="BuildException">The folder has no project, more than one, or no World Editor map.</exception>
        public static MapProject Find(string path)
        {
            var full = Path.GetFullPath(path);
            string folder;
            string projectFile;
            if (string.Equals(Path.GetExtension(full), ".csproj", StringComparison.OrdinalIgnoreCase))
            {
                folder = Path.GetDirectoryName(full)!;
                projectFile = full;
            }
            else
            {
                folder = Path.TrimEndingDirectorySeparator(full);
                var names = Directory.Exists(folder)
                    ? Directory.GetFiles(folder, "*.csproj", SearchOption.TopDirectoryOnly)
                        .Select(file => Path.GetFileName(file))
                        .OrderBy(name => name, StringComparer.Ordinal)
                        .ToList()
                    : new List<string>();
                if (names.Count == 0)
                {
                    throw new BuildException($"no C# project in {folder}");
                }

                if (names.Count > 1)
                {
                    throw new BuildException($"more than one C# project in {folder}: {string.Join(", ", names)}");
                }

                projectFile = Path.Combine(folder, names[0]);
            }

            var project = new MapProject(folder, projectFile);
            if (!File.Exists(Path.Combine(project.MapFolder, "war3map.w3i")))
            {
                throw new BuildException(@"no World Editor map: map\war3map.w3i is missing");
            }

            return project;
        }
    }
}
