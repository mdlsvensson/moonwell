using Moonwell.Generator;

if (args.Length != 3)
{
    Console.Error.WriteLine("usage: Moonwell.Generator <folder with common.j, blizzard.j, common.ai> <repository root> <game version>");
    return 2;
}

var (scriptsFolder, root, gameVersion) = (args[0], args[1], args[2]);
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
