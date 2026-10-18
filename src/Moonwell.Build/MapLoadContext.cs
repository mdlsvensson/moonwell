using System.Reflection;
using System.Runtime.Loader;

namespace Moonwell.Build
{
    // Loads from bytes, so that no file stays locked, and finds the map's own copies of Moonwell beside it.
    internal sealed class MapLoadContext : AssemblyLoadContext
    {
        private readonly string folder;

        public MapLoadContext(string folder)
            : base("moonwell-map", true)
        {
            this.folder = folder;
        }

        public Assembly Open(string path)
        {
            return Read(path);
        }

        protected override Assembly? Load(AssemblyName assemblyName)
        {
            var path = Path.Combine(folder, assemblyName.Name + ".dll");
            return File.Exists(path) ? Read(path) : null;
        }

        private Assembly Read(string path)
        {
            using var assembly = new MemoryStream(File.ReadAllBytes(path));
            var symbols = Path.ChangeExtension(path, ".pdb");
            if (File.Exists(symbols))
            {
                using var pdb = new MemoryStream(File.ReadAllBytes(symbols));
                return LoadFromStream(assembly, pdb);
            }

            return LoadFromStream(assembly);
        }
    }
}
