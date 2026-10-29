using Moonwell.GameData;
using Moonwell.MapFiles;

namespace Moonwell.Build
{
    // One object-data file of the source map as the build holds it: Moonwell.MapFiles' file, and whether it existed when
    // the build read it. A file the map lacks is an empty one; the build writes it only when it holds an object.
    internal sealed class SourceObjectFile
    {
        private SourceObjectFile(ObjectFile file, bool existed)
        {
            File = file;
            Existed = existed;
        }

        /// <summary>The file's objects and modifications; the build edits it in place.</summary>
        public ObjectFile File { get; }

        /// <summary>Whether the file existed when it was read.</summary>
        public bool Existed { get; }

        /// <summary>Whether the file would hold any object.</summary>
        public bool IsEmpty => File.Original.Count == 0 && File.Custom.Count == 0;

        /// <summary>Every object of the file: the changed standard objects, then the new ones.</summary>
        public IEnumerable<ObjectEntry> Entries => File.Original.Concat(File.Custom);

        /// <summary>Reads an object-data file; a missing file is an empty format 3 file.</summary>
        /// <param name="path">The file.</param>
        /// <param name="kind">The kind the file holds.</param>
        /// <exception cref="BuildException">The file cannot be read.</exception>
        public static SourceObjectFile Read(string path, Kind kind)
        {
            if (!System.IO.File.Exists(path))
            {
                return new SourceObjectFile(ObjectFile.Empty(kind), false);
            }

            var name = Path.GetFileName(path);
            try
            {
                return new SourceObjectFile(ObjectFile.ReadLenient(kind, System.IO.File.ReadAllBytes(path), name), true);
            }
            catch (ObjectFileException failure)
            {
                throw new BuildException($"cannot read {name}: {(failure.InnerException ?? failure).Message}");
            }
            catch (IOException failure)
            {
                throw new BuildException($"cannot read {name}: {failure.Message}");
            }
        }

        /// <summary>The id an object has: its own, else the standard object's.</summary>
        public static string IdOf(ObjectEntry entry)
        {
            return entry.Id.Length > 0 ? entry.Id : entry.BaseId;
        }

        /// <summary>A modification's value as the CLR value War3Net holds: an int, a float, a string, a bool or a char.</summary>
        public static object ValueOf(FieldMod mod)
        {
            return mod.Value switch
            {
                FieldValue.Int number => number.Value,
                FieldValue.Real real => real.Value,
                FieldValue.Unreal unreal => unreal.Value,
                FieldValue.Text text => text.Value,
                FieldValue.Bool flag => flag.Value,
                FieldValue.Char letter => letter.Value,
                _ => throw new InvalidOperationException("Unknown value type."),
            };
        }

        /// <summary>Writes the file's bytes.</summary>
        /// <param name="path">The file; its folder exists.</param>
        public void Write(string path)
        {
            System.IO.File.WriteAllBytes(path, File.ToBytes());
        }
    }
}
