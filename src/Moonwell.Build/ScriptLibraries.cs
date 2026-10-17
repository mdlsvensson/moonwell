using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
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

        /// <summary>Writes CSharp.lua's core files and the Lua of the map's copies of Moonwell.Natives and Moonwell into a folder.</summary>
        /// <param name="targetPath">The map's built assembly; the two Moonwell libraries are next to it.</param>
        /// <param name="folder">The folder the Moonwell files are written to; it is created when missing.</param>
        /// <returns>The paths of the files to link, in order: the core files, then <c>types.lua</c>, then <c>moonwell.lua</c>.</returns>
        /// <exception cref="BuildException">A library or its embedded Lua is missing.</exception>
        public static List<string> Extract(string targetPath, string folder)
        {
            var beside = Path.GetDirectoryName(Path.GetFullPath(targetPath))!;
            var natives = ReadResource(Path.Combine(beside, NativesLibrary), NativesResource, targetPath);
            var moonwell = ReadResource(Path.Combine(beside, MoonwellLibrary), MoonwellResource, targetPath);

            var files = CoreSystemProvider.GetCoreSystemFiles()
                .Select(file => Path.Combine(AppContext.BaseDirectory, file))
                .ToList();
            Directory.CreateDirectory(folder);
            files.Add(Write(folder, "types.lua", natives));
            files.Add(Write(folder, "moonwell.lua", moonwell));
            return files;
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
