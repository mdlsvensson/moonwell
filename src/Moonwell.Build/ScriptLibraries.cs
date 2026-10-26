using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
using System.Text.RegularExpressions;
using CSharpLua.CoreSystem;

namespace Moonwell.Build
{
    /// <summary>Collects the Lua files the map's script is linked with.</summary>
    public static class ScriptLibraries
    {
        private const string NativesLibrary = "Moonwell.Natives.dll";
        private const string NativesResource = "Moonwell.Natives.types.lua";
        private const string MoonwellLibrary = "Moonwell.dll";
        private const string MoonwellResource = "Moonwell.moonwell.lua";

        /// <summary>The groups of CSharp.lua's core files a map rarely uses, by their folder: numerics (vectors, matrices, quaternions, complex numbers) and threading (tasks, async, threads). No other core file uses either.</summary>
        internal static readonly string[] TrimmedGroups = { "Numerics", "Threading" };

        /// <summary>Writes CSharp.lua's core files and the Lua of the map's copies of Moonwell.Natives and Moonwell into a folder.</summary>
        /// <param name="targetPath">The map's built assembly; the two Moonwell libraries are next to it.</param>
        /// <param name="folder">The folder the Moonwell files are written to; it is created when missing.</param>
        /// <returns>
        /// The paths of the files to link, in order: the core files, then <c>types.lua</c>, then <c>moonwell.lua</c>. A file
        /// of <see cref="TrimmedGroups"/> is a copy in the folder between two marker lines, for <see cref="Trim"/>.
        /// </returns>
        /// <exception cref="BuildException">A library or its embedded Lua is missing.</exception>
        public static List<string> Extract(string targetPath, string folder)
        {
            var beside = Path.GetDirectoryName(Path.GetFullPath(targetPath))!;
            var natives = ReadResource(Path.Combine(beside, NativesLibrary), NativesResource, targetPath);
            var moonwell = ReadResource(Path.Combine(beside, MoonwellLibrary), MoonwellResource, targetPath);

            Directory.CreateDirectory(folder);
            var files = new List<string>();
            foreach (var file in CoreSystemProvider.GetCoreSystemFiles())
            {
                var path = Path.Combine(AppContext.BaseDirectory, file);
                var group = GroupOf(file);
                if (group == null)
                {
                    files.Add(path);
                    continue;
                }

                var copy = Path.Combine(folder, "CoreSystem", group, Path.GetFileName(file));
                Directory.CreateDirectory(Path.GetDirectoryName(copy)!);
                File.WriteAllText(copy, Marked(group, File.ReadAllText(path)));
                files.Add(copy);
            }

            files.Add(Write(folder, "types.lua", natives));
            files.Add(Write(folder, "moonwell.lua", moonwell));
            return files;
        }

        /// <summary>A core file's text between the two marker lines of its group, which <see cref="Trim"/> finds in the script.</summary>
        internal static string Marked(string group, string text)
        {
            return Begin(group) + "\n" + text.Replace("\r\n", "\n").TrimEnd('\n') + "\n" + End(group) + "\n";
        }

        /// <summary>
        /// Leaves out of a linked script each group of <see cref="TrimmedGroups"/> that nothing else in the script names a
        /// name of (a name a file of the group defines, or the group's namespace, <c>System.Numerics</c> or
        /// <c>System.Threading</c>), and takes the marker lines out of the groups it keeps.
        /// </summary>
        /// <param name="script">The script CSharp.lua linked, each library file in its own <c>do ... end</c> block.</param>
        /// <returns>The script without the groups nothing uses.</returns>
        public static string Trim(string script)
        {
            foreach (var group in TrimmedGroups)
            {
                // A file's block, from its do to its end, and the blank line after it; the linker's own line ends are \r\n.
                var blocks = new Regex(@"do\r?\n" + Regex.Escape(Begin(group)) + @"\n(.*?)\n" + Regex.Escape(End(group)) + @"(?:\r?\n)+end\r?\n(?:\r?\n)?", RegexOptions.Singleline);
                var found = blocks.Matches(script);
                if (found.Count == 0)
                {
                    continue;
                }

                // CSharp.lua names what the code uses through an alias of its namespace at the top of the file
                // (local SystemNumerics = System.Numerics), so the group's namespace is one of its names too.
                var rest = blocks.Replace(script, string.Empty);
                var names = found.SelectMany(block => DefinedNames(block.Groups[1].Value)).Append("System." + group).Distinct();
                var used = names.Any(name => Names(rest, name));
                script = used
                    ? script.Replace(Begin(group) + "\n", string.Empty, StringComparison.Ordinal).Replace(End(group) + "\n", string.Empty, StringComparison.Ordinal)
                    : rest;
            }

            return script;
        }

        private static string Begin(string group) => "local _ = \"moonwell trim " + group + "\"";

        private static string End(string group) => "local _ = \"moonwell trim " + group + " end\"";

        // The group of a core file by its folder (CoreSystem/Numerics/Vector2.lua), or null for one of no group.
        private static string? GroupOf(string file)
        {
            var parts = file.Replace('\\', '/').Split('/');
            return parts.Length >= 3 && TrimmedGroups.Contains(parts[parts.Length - 2]) ? parts[parts.Length - 2] : null;
        }

        // The names a core file defines: the full names it defines classes under and the System fields it sets.
        private static IEnumerable<string> DefinedNames(string block)
        {
            foreach (Match match in Regex.Matches(block, @"(?:define|defStc|defEnum|defInf|defCls)\(""([A-Za-z0-9_.]+)""|System\.([A-Za-z0-9_]+)\s*=[^=]|function System\.([A-Za-z0-9_]+)"))
            {
                for (var group = 1; group <= 3; group++)
                {
                    if (match.Groups[group].Success)
                    {
                        yield return group == 1 ? match.Groups[group].Value : "System." + match.Groups[group].Value;
                    }
                }
            }
        }

        // Whether the text names a dotted name as a whole, not as the start of a longer one.
        private static bool Names(string text, string name)
        {
            return Regex.IsMatch(text, Regex.Escape(name) + @"(?![A-Za-z0-9_])");
        }

        private static string Write(string folder, string name, byte[] content)
        {
            var path = Path.Combine(folder, name);
            File.WriteAllBytes(path, content);
            return path;
        }

        private static byte[] ReadResource(string libraryPath, string resource, string targetPath)
        {
            var missing = new BuildException($"{Path.GetFileName(libraryPath)} is not next to {targetPath}");
            if (!File.Exists(libraryPath))
            {
                throw missing;
            }

            using var stream = new MemoryStream(File.ReadAllBytes(libraryPath));
            using var pe = new PEReader(stream);
            if (!pe.HasMetadata || pe.PEHeaders.CorHeader == null)
            {
                throw missing;
            }

            var metadata = pe.GetMetadataReader();
            foreach (var handle in metadata.ManifestResources)
            {
                var entry = metadata.GetManifestResource(handle);
                if (metadata.GetString(entry.Name) != resource || !entry.Implementation.IsNil)
                {
                    continue;
                }

                var section = pe.GetSectionData(pe.PEHeaders.CorHeader.ResourcesDirectory.RelativeVirtualAddress);
                var reader = section.GetReader((int)entry.Offset, section.Length - (int)entry.Offset);
                var length = reader.ReadInt32();
                return reader.ReadBytes(length);
            }

            throw new BuildException($"{Path.GetFileName(libraryPath)} next to {targetPath} has no {resource}");
        }
    }
}
