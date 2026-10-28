using Moonwell.GameData;

namespace Moonwell.MapFiles.Tests;

/// <summary>The map files in Fixtures, copied next to the test assembly.</summary>
internal static class Fixtures
{
    /// <summary>The folder that holds one folder per fixture.</summary>
    public static string Root => Path.Combine(AppContext.BaseDirectory, "Fixtures");

    /// <summary>The fixture folders: World Editor 3.0's two saves, Moonwell's MyMap build and the build tests' World Editor save.</summary>
    public static readonly string[] Folders = ["we-3.0", "we-heavy", "moonwell-mymap", "WorldEditorObjects"];

    /// <summary>Every object file of every fixture, as "folder/file".</summary>
    public static IEnumerable<string> ObjectFiles()
    {
        foreach (var folder in Folders)
        {
            var files = Directory.GetFiles(Path.Combine(Root, folder))
                .Where(path => KindOf(path) is not null)
                .Select(path => folder + "/" + Path.GetFileName(path))
                .Order(StringComparer.Ordinal);
            foreach (var file in files)
            {
                yield return file;
            }
        }
    }

    /// <summary>Every object file of every fixture, as theory data.</summary>
    public static TheoryData<string> ObjectFileData()
    {
        var data = new TheoryData<string>();
        foreach (var file in ObjectFiles())
        {
            data.Add(file);
        }

        return data;
    }

    /// <summary>The bytes of a fixture file, named "folder/file".</summary>
    public static byte[] Bytes(string file) => File.ReadAllBytes(Path.Combine(Root, file));

    /// <summary>The object kind of a file, from its extension; null for other files.</summary>
    public static Kind? KindOf(string path)
    {
        var extension = Path.GetExtension(path).TrimStart('.');
        foreach (var kind in Enum.GetValues<Kind>())
        {
            if (Kinds.Extension(kind) == extension)
            {
                return kind;
            }
        }

        return null;
    }

    /// <summary>Reads a fixture file with <see cref="ObjectFile.ReadExact"/>.</summary>
    public static ObjectFile Read(string file) => ObjectFile.ReadExact(KindOf(file)!.Value, Bytes(file));
}
