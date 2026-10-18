using System.Xml.Linq;

namespace Moonwell.Build
{
    /// <summary>The copy of a map project, in <c>build\marked</c>, whose files carry <see cref="LineMarks"/>; CSharp.lua compiles it.</summary>
    public sealed class MarkedProject
    {
        private readonly string projectFolder;

        private MarkedProject(string folder, string projectFile, IReadOnlyList<string> files, string projectFolder)
        {
            Folder = folder;
            ProjectFile = projectFile;
            Files = files;
            this.projectFolder = projectFolder;
        }

        /// <summary>The copy's folder, <c>build\marked</c>.</summary>
        public string Folder { get; }

        /// <summary>The copy's <c>.csproj</c>, its project references made absolute.</summary>
        public string ProjectFile { get; }

        /// <summary>The marked files, relative to the project folder with <c>/</c>, in order: the first is file 1.</summary>
        public IReadOnlyList<string> Files { get; }

        /// <summary>
        /// Empties <c>build\marked</c> and copies into it the project file and every <c>*.cs</c> CSharp.lua compiles (all under the
        /// project folder but the top-level <c>bin</c> and <c>obj</c>), marking the files outside <c>build</c>.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="symbols">The preprocessor symbols CSharp.lua compiles with.</param>
        /// <returns>The copy.</returns>
        public static MarkedProject Stage(MapProject project, IEnumerable<string> symbols)
        {
            var folder = Path.Combine(project.BuildFolder, "marked");
            if (Directory.Exists(folder))
            {
                Directory.Delete(folder, true);
            }

            Directory.CreateDirectory(folder);
            var symbolList = symbols.ToList();
            var sources = SourceFiles(project.Folder, folder)
                .Select(path => Path.GetRelativePath(project.Folder, path).Replace('\\', '/'))
                .OrderBy(path => path, StringComparer.Ordinal)
                .ToList();
            var marked = new List<string>();
            foreach (var relative in sources)
            {
                var text = File.ReadAllText(Path.Combine(project.Folder, relative));
                if (!relative.StartsWith("build/", StringComparison.OrdinalIgnoreCase))
                {
                    marked.Add(relative);
                    text = LineMarks.Mark(text, marked.Count, symbolList);
                }

                var target = Path.Combine(folder, relative);
                Directory.CreateDirectory(Path.GetDirectoryName(target)!);
                File.WriteAllText(target, text);
            }

            var projectFile = Path.Combine(folder, Path.GetFileName(project.ProjectFile));
            WriteProjectFile(project, projectFile);
            return new MarkedProject(folder, projectFile, marked, project.Folder);
        }

        /// <summary>A message from CSharp.lua with the copy's paths changed back to the project's.</summary>
        public string Unstage(string message)
        {
            return message.Replace(Folder, projectFolder, StringComparison.OrdinalIgnoreCase);
        }

        private static IEnumerable<string> SourceFiles(string projectFolder, string markedFolder)
        {
            foreach (var file in Directory.EnumerateFiles(projectFolder, "*.cs", SearchOption.TopDirectoryOnly))
            {
                yield return file;
            }

            foreach (var sub in Directory.EnumerateDirectories(projectFolder))
            {
                var name = Path.GetFileName(sub);
                if (name.Equals("bin", StringComparison.OrdinalIgnoreCase) || name.Equals("obj", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                foreach (var file in Directory.EnumerateFiles(sub, "*.cs", SearchOption.AllDirectories))
                {
                    if (!file.StartsWith(markedFolder + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
                    {
                        yield return file;
                    }
                }
            }
        }

        // The copy is deeper than the project, so its relative project references would point elsewhere.
        private static void WriteProjectFile(MapProject project, string path)
        {
            var document = XDocument.Load(project.ProjectFile, LoadOptions.PreserveWhitespace);
            foreach (var reference in document.Descendants().Where(element => element.Name.LocalName == "ProjectReference"))
            {
                var include = reference.Attribute("Include");
                if (include != null && !Path.IsPathRooted(include.Value))
                {
                    include.Value = Path.GetFullPath(Path.Combine(project.Folder, include.Value));
                }
            }

            document.Save(path);
        }
    }
}
