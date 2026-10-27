using Moonwell.GameData;
using Moonwell.Generator;
using Moonwell.Generator.Objects;

const string Usage = """
    usage:
      Moonwell.Generator natives <folder with common.j, blizzard.j, common.ai> <repository root> <game version>
      Moonwell.Generator objects <war3.w3mod folder> <repository root> <game version>
    """;

if (args.Length != 4 || (args[0] != "natives" && args[0] != "objects"))
{
    Console.Error.WriteLine(Usage);
    return 2;
}

var (command, source, root, gameVersion) = (args[0], args[1], args[2], args[3]);
return command == "natives" ? Natives(source, root, gameVersion) : Objects(source, root, gameVersion);

static int Natives(string scriptsFolder, string root, string gameVersion)
{
    var scripts = Jass.Read(
        File.ReadAllText(Path.Combine(scriptsFolder, "common.j")),
        File.ReadAllText(Path.Combine(scriptsFolder, "blizzard.j")));
    LuaExtras.AddTo(scripts, File.ReadAllText(Path.Combine(scriptsFolder, "common.ai")));

    var natives = Path.Combine(root, "src", "Moonwell.Natives");
    Directory.CreateDirectory(Path.Combine(natives, "Generated"));
    File.WriteAllText(Path.Combine(natives, "Generated", "Types.g.cs"), NativesWriter.WriteTypes(scripts, gameVersion));
    File.WriteAllText(Path.Combine(natives, "Generated", "Natives.g.cs"), NativesWriter.WriteNatives(scripts, gameVersion));
    File.WriteAllText(Path.Combine(natives, "Generated", "types.lua"), NativesWriter.WriteLuaTypes(scripts, gameVersion));

    Console.WriteLine($"{scripts.Types.Count} types, {scripts.Functions.Count} functions, {scripts.Globals.Count} globals");
    return 0;
}

static int Objects(string exportFolder, string root, string gameVersion)
{
    var runtime = RuntimeMembers.Read(MoonwellDll(root));
    var files = GeneratedObjects.Make(GameTables.OpenExport(exportFolder), runtime, gameVersion);

    var generated = Path.Combine(root, "src", "Moonwell.Objects", "Generated");
    if (Directory.Exists(generated))
    {
        Directory.Delete(generated, true);
    }

    long source = 0;
    long values = 0;
    foreach (var (path, text) in files)
    {
        var target = Path.Combine(generated, path);
        Directory.CreateDirectory(Path.GetDirectoryName(target)!);
        File.WriteAllText(target, text);
        var size = new FileInfo(target).Length;
        if (path.EndsWith(".json", StringComparison.Ordinal))
        {
            values += size;
        }
        else
        {
            source += size;
        }
    }

    Console.WriteLine($"{files.Count} files: {source:N0} bytes of C#, {values:N0} bytes of values");
    return 0;
}

static string MoonwellDll(string root)
{
    string? newest = null;
    foreach (var configuration in new[] { "Debug", "Release" })
    {
        var path = Path.Combine(root, "src", "Moonwell", "bin", configuration, "net10.0", "Moonwell.dll");
        if (File.Exists(path) && (newest == null || File.GetLastWriteTimeUtc(path) > File.GetLastWriteTimeUtc(newest)))
        {
            newest = path;
        }
    }

    return newest ?? Path.Combine(root, "src", "Moonwell", "bin", "Debug", "net10.0", "Moonwell.dll");
}
