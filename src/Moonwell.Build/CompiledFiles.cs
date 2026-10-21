namespace Moonwell.Build
{
    /// <summary>Which files a map project's code is compiled from and against: the one place the build decides.</summary>
    internal static class CompiledFiles
    {
        /// <summary>
        /// The C# files <c>dotnet build</c> and CSharp.lua compile: every <c>*.cs</c> under a folder but the top-level <c>bin</c> and
        /// <c>obj</c> (their files are the compiler's own) and the folders given.
        /// </summary>
        /// <param name="folder">The project folder, or a copy of it.</param>
        /// <param name="skipped">Full paths of folders under it to leave out, such as the build folder.</param>
        /// <returns>The files' full paths.</returns>
        public static IEnumerable<string> Sources(string folder, params string[] skipped)
        {
            bool Skipped(string path) => skipped.Any(one => path.Equals(one, StringComparison.OrdinalIgnoreCase)
                || path.StartsWith(one + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase));

            foreach (var file in Directory.EnumerateFiles(folder, "*.cs", SearchOption.TopDirectoryOnly))
            {
                yield return file;
            }

            foreach (var sub in Directory.EnumerateDirectories(folder))
            {
                var name = Path.GetFileName(sub);
                if (name.Equals("bin", StringComparison.OrdinalIgnoreCase) || name.Equals("obj", StringComparison.OrdinalIgnoreCase) || Skipped(sub))
                {
                    continue;
                }

                foreach (var file in Directory.EnumerateFiles(sub, "*.cs", SearchOption.AllDirectories))
                {
                    if (!Skipped(file))
                    {
                        yield return file;
                    }
                }
            }
        }

        /// <summary>The libraries <c>dotnet build</c> put beside the map's assembly, which the map's code references.</summary>
        /// <param name="assemblyPath">The map's built assembly.</param>
        /// <returns>The libraries' paths, the assembly itself left out.</returns>
        public static IEnumerable<string> LibrariesBeside(string assemblyPath)
        {
            var assembly = Path.GetFullPath(assemblyPath);
            return Directory.EnumerateFiles(Path.GetDirectoryName(assembly)!, "*.dll")
                .Where(path => !string.Equals(Path.GetFullPath(path), assembly, StringComparison.OrdinalIgnoreCase));
        }
    }
}
