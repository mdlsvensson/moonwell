using Moonwell.GameData;
using War3Net.Build.Extensions;
using War3Net.Build.Object;
using Xunit.Abstractions;

namespace Moonwell.MapFiles.Tests;

public sealed class ObjectFileTests(ITestOutputHelper output)
{
    public static TheoryData<string> ObjectFiles => Fixtures.ObjectFileData();

    [Theory]
    [MemberData(nameof(ObjectFiles))]
    public void Every_fixture_file_round_trips(string file)
    {
        var bytes = Fixtures.Bytes(file);

        var objectFile = Fixtures.Read(file);

        Assert.Equal(bytes, objectFile.ToBytes());
    }

    [Fact]
    public void Every_fixture_folder_has_object_files()
    {
        var files = Fixtures.ObjectFiles().ToList();

        Assert.Equal(14, files.Count(file => file.StartsWith("we-3.0/", StringComparison.Ordinal)));
        Assert.Equal(12, files.Count(file => file.StartsWith("moonwell-mymap/", StringComparison.Ordinal)));
        Assert.Equal(14, files.Count(file => file.StartsWith("we-heavy/", StringComparison.Ordinal)));
        Assert.Equal(14, files.Count(file => file.StartsWith("WorldEditorObjects/", StringComparison.Ordinal)));
    }

    [Theory]
    [MemberData(nameof(ObjectFiles))]
    public void Every_fixture_file_reads_and_writes_back_the_same_bytes(string file)
    {
        var bytes = Fixtures.Bytes(file);

        var objectFile = ObjectFile.Read(Fixtures.KindOf(file)!.Value, bytes);

        Assert.Equal(bytes, objectFile.ToBytes());
    }

    [Fact]
    public void A_bool_value_is_read_and_written_back()
    {
        // World Editor never writes War3Net's obsolete bool and char value types, so the file is built with War3Net.
        var bytes = BoolAndCharUnitFile();

        var read = ObjectFile.Read(Kind.Unit, bytes);

        var entry = Assert.Single(read.Custom);
        Assert.Equal(new FieldValue.Bool(true), entry.Find("ubol", 0)!.Value);
        Assert.Equal(new FieldValue.Char('x'), entry.Find("uchr", 0)!.Value);
        Assert.Equal(bytes, read.ToBytes());

        // A value set by hand is written the same way War3Net wrote it.
        entry.Set("ubol", 0, 0, new FieldValue.Bool(false));
        entry.Set("uchr", 0, 0, new FieldValue.Char('y'));
        var again = ObjectFile.Read(Kind.Unit, read.ToBytes()).Custom[0];
        Assert.Equal(new FieldValue.Bool(false), again.Find("ubol", 0)!.Value);
        Assert.Equal(new FieldValue.Char('y'), again.Find("uchr", 0)!.Value);

        // The exact read still refuses what the editor cannot save, and names the field.
        var problem = Assert.Throws<ObjectFileException>(() => ObjectFile.ReadExact(Kind.Unit, bytes));
        Assert.Contains("ubol", problem.Message);
        Assert.Contains("value type 4", problem.Message);
        Assert.Null(problem.Parsed);
    }

    [Fact]
    public void Set_flags_are_shown()
    {
        var units = Fixtures.Read("we-3.0/war3map.w3u");

        var entry = Assert.Single(units.Custom);

        Assert.Equal([0], entry.SetFlags);
        Assert.Equal(entry.SetFlags.Count, entry.SetCount);
    }

    [Fact]
    public void We_fixture_values()
    {
        var abilities = Fixtures.Read("we-3.0/war3mapSkin.w3a");

        Assert.Equal(Kind.Ability, abilities.Kind);
        Assert.Empty(abilities.Original);
        var entry = Assert.Single(abilities.Custom);
        Assert.Equal("ANab", entry.BaseId);
        Assert.Equal("A000", entry.Id);
        Assert.Equal(1, entry.SetCount);
        var mod = Assert.Single(entry.Mods);
        Assert.Equal("anam", mod.Field);
        Assert.Equal(0, mod.Level);
        Assert.Equal(0, mod.Pointer);
        Assert.Equal(new FieldValue.Text("TRIGSTR_016"), mod.Value);
        Assert.Same(mod, entry.Find("anam", 0));

        var upgrades = Fixtures.Read("we-3.0/war3mapSkin.w3q");

        var upgrade = Assert.Single(upgrades.Custom);
        Assert.Equal("Rhme", upgrade.BaseId);
        Assert.Equal("R000", upgrade.Id);
        var name = upgrade.Find("gnam", 1);
        Assert.NotNull(name);
        Assert.Equal(1, name.Level);
        Assert.Null(upgrade.Find("gnam", 0));
    }

    [Theory]
    [MemberData(nameof(ObjectFiles))]
    public void Truncated_file_is_a_problem(string file)
    {
        var bytes = Fixtures.Bytes(file);
        var kind = Fixtures.KindOf(file)!.Value;

        for (var length = 0; length < bytes.Length; length++)
        {
            var cut = bytes[..length];

            var problem = Assert.Throws<ObjectFileException>(() => ObjectFile.ReadExact(kind, cut));

            Assert.Contains("war3map." + Kinds.Extension(kind), problem.Message);
            Assert.Null(problem.Parsed);

            // The lenient read fails the same way: War3Net cannot read the file.
            var lenient = Assert.Throws<ObjectFileException>(() => ObjectFile.Read(kind, cut));
            Assert.Contains("War3Net cannot read war3map." + Kinds.Extension(kind), lenient.Message);
        }
    }

    [Theory]
    [MemberData(nameof(ObjectFiles))]
    public void Trailing_table_round_trips(string file)
    {
        // Some tools write a third, empty table; War3Net 6.1.2-moonwell.4 keeps it.
        var bytes = Fixtures.Bytes(file);
        var kind = Fixtures.KindOf(file)!.Value;
        byte[] longer = [.. bytes, 0, 0, 0, 0];

        var read = ObjectFile.ReadExact(kind, longer);

        Assert.Equal(longer, read.ToBytes());
    }

    [Theory]
    [MemberData(nameof(ObjectFiles))]
    public void Lossy_write_back_is_a_problem(string file)
    {
        // War3Net writes every known file back exactly; the guard is tested with a write-back that drops 4 bytes.
        var bytes = Fixtures.Bytes(file);
        var kind = Fixtures.KindOf(file)!.Value;

        var problem = Assert.Throws<ObjectFileException>(
            () => ObjectFile.ReadExact(kind, bytes, null, read => read.ToBytes()[..^4]));

        Assert.Contains("exactly", problem.Message);
        Assert.Contains("drop the last 4 bytes", problem.Message);
        Assert.Contains("war3map." + Kinds.Extension(kind), problem.Message);
        output.WriteLine(problem.Message);

        // War3Net read it, so what it read is kept for showing (never for saving).
        var parsed = problem.Parsed;
        Assert.NotNull(parsed);
        var read = ObjectFile.ReadExact(kind, bytes);
        Assert.Equal(kind, parsed.Kind);
        Assert.Equal(read.Original.Count, parsed.Original.Count);
        Assert.Equal(read.Custom.Select(entry => entry.Id), parsed.Custom.Select(entry => entry.Id));
        Assert.Equal(
            read.Original.Concat(read.Custom).SelectMany(entry => entry.Mods).Select(mod => mod.Value),
            parsed.Original.Concat(parsed.Custom).SelectMany(entry => entry.Mods).Select(mod => mod.Value));
    }

    [Fact]
    public void Write_back_differing_inside_is_a_problem()
    {
        var bytes = Fixtures.Bytes("we-3.0/war3mapSkin.w3a");

        var problem = Assert.Throws<ObjectFileException>(() => ObjectFile.ReadExact(
            Kind.Ability,
            bytes,
            "war3mapSkin.w3a",
            read =>
            {
                var written = read.ToBytes();
                written[10] ^= 0xFF;
                return written;
            }));

        Assert.Contains("war3mapSkin.w3a back exactly", problem.Message);
        Assert.Contains("from offset 10", problem.Message);
        Assert.NotNull(problem.Parsed);
    }

    [Fact]
    public void Non_utf8_name_round_trips()
    {
        var bytes = Fixtures.Bytes("we-3.0/war3mapSkin.w3a");
        var at = IndexOf(bytes, "TRIGSTR_016"u8);
        Assert.True(at > 0);
        bytes[at] = 0xFF;

        var read = ObjectFile.ReadExact(Kind.Ability, bytes, "war3mapSkin.w3a");

        // War3Net shows the byte as a replacement character and keeps it while the value is unchanged.
        Assert.Equal(bytes, read.ToBytes());
        var entry = read.Custom[0];
        Assert.Equal(new FieldValue.Text("�RIGSTR_016"), entry.Find("anam", 0)!.Value);

        // Editing another field of the same object leaves the name's bytes as they were.
        entry.Set("ahky", 0, 0, new FieldValue.Text("Q"));
        var written = read.ToBytes();
        Assert.True(IndexOf(written, [0xFF, .. "RIGSTR_016"u8]) > 0);
    }

    [Fact]
    public void Edit_then_reread()
    {
        var bytes = Fixtures.Bytes("moonwell-mymap/war3map.w3u");
        var units = ObjectFile.ReadExact(Kind.Unit, bytes);
        var before = Snapshot(units);
        var paladin = units.Custom.Single(entry => entry.Id == "H001");
        var oldHitPoints = paladin.Find("uhpm", 0)!;
        var index = IndexOf(paladin.Mods, oldHitPoints);
        var mage = units.Custom.Single(entry => entry.Id == "H002");
        Assert.Null(mage.Find("uhpm", 0));

        var replaced = paladin.Set("uhpm", 0, 0, new FieldValue.Int(450));
        var added = mage.Set("uhpm", 0, 0, new FieldValue.Int(275));

        Assert.Same(oldHitPoints, replaced);
        Assert.Equal(index, IndexOf(paladin.Mods, replaced));
        Assert.Same(added, mage.Mods[^1]);

        var again = ObjectFile.ReadExact(Kind.Unit, units.ToBytes());

        var after = Snapshot(again);
        Assert.Contains("H001 uhpm 0 0 Int { Value = 450 }", after);
        Assert.Contains("H002 uhpm 0 0 Int { Value = 275 }", after);
        var expected = before
            .Select(line => line == "H001 uhpm 0 0 Int { Value = 300 }" ? "H001 uhpm 0 0 Int { Value = 450 }" : line)
            .ToList();
        var mageEnd = expected.FindLastIndex(line => line.StartsWith("H002 ", StringComparison.Ordinal));
        expected.Insert(mageEnd + 1, "H002 uhpm 0 0 Int { Value = 275 }");
        Assert.Equal(expected, after);
        Assert.Equal(bytes.Length + 16, again.ToBytes().Length);
    }

    [Fact]
    public void Removed_mod_and_entry_are_gone_after_reread()
    {
        var units = Fixtures.Read("moonwell-mymap/war3map.w3u");
        var paladin = units.Custom.Single(entry => entry.Id == "H001");
        paladin.Remove(paladin.Find("uhpm", 0)!);
        units.Remove(units.Custom.Single(entry => entry.Id == "H002"));

        var again = ObjectFile.ReadExact(Kind.Unit, units.ToBytes());

        Assert.Null(again.Custom.Single(entry => entry.Id == "H001").Find("uhpm", 0));
        Assert.DoesNotContain(again.Custom, entry => entry.Id == "H002");
    }

    [Fact]
    public void Removed_entry_goes_back_in_place()
    {
        var bytes = Fixtures.Bytes("moonwell-mymap/war3map.w3u");
        var units = ObjectFile.ReadExact(Kind.Unit, bytes);
        var first = units.Custom[0];
        var original = units.AddOriginal("hfoo");
        units.Remove(first);
        units.Remove(original);

        units.Insert(0, first);

        Assert.Same(first, units.Custom[0]);
        Assert.Equal(bytes, units.ToBytes());
        units.Insert(units.Original.Count, original);
        Assert.Same(original, units.Original[^1]);
        Assert.Throws<ArgumentException>(() => units.Insert(0, first));
        Assert.Throws<ArgumentException>(
            () => units.Insert(0, ObjectFile.Empty(Kind.Ability).AddCustom("ANab", "A000")));
    }

    [Fact]
    public void Removed_mod_goes_back_in_place()
    {
        var bytes = Fixtures.Bytes("moonwell-mymap/war3map.w3u");
        var units = ObjectFile.ReadExact(Kind.Unit, bytes);
        var peasant = units.Original.Single(entry => entry.BaseId == "hpea");
        var first = peasant.Mods[0];
        peasant.Remove(first);

        peasant.Insert(0, first);

        Assert.Same(first, peasant.Mods[0]);
        Assert.Equal(bytes, units.ToBytes());
        Assert.Throws<ArgumentException>(() => peasant.Insert(0, first));
        Assert.Throws<ArgumentOutOfRangeException>(
            () => peasant.Insert(peasant.Mods.Count + 2, ObjectFile.Empty(Kind.Unit).AddOriginal("hfoo")
                .Set("uhpm", 0, 0, new FieldValue.Int(1))));
        var ability = ObjectFile.Empty(Kind.Ability).AddOriginal("AHhb");
        var levelMod = ability.Set("acdn", 1, 0, new FieldValue.Real(1));
        ability.Remove(levelMod);
        Assert.Throws<ArgumentException>(() => peasant.Insert(0, levelMod));
    }

    [Fact]
    public void New_entries_write_as_world_editor_does()
    {
        var abilities = ObjectFile.Empty(Kind.Ability);
        abilities.AddCustom("ANab", "A000").Set("anam", 0, 0, new FieldValue.Text("TRIGSTR_016"));
        var upgrades = ObjectFile.Empty(Kind.Upgrade);
        upgrades.AddCustom("Rhme", "R000").Set("gnam", 1, 0, new FieldValue.Text("TRIGSTR_018"));
        var units = ObjectFile.Empty(Kind.Unit);
        units.AddCustom("hpea", "h000");

        Assert.Equal(Fixtures.Bytes("we-3.0/war3mapSkin.w3a"), abilities.ToBytes());
        Assert.Equal(Fixtures.Bytes("we-3.0/war3mapSkin.w3q"), upgrades.ToBytes());
        Assert.Equal(Fixtures.Bytes("we-3.0/war3map.w3u"), units.ToBytes());
        Assert.Equal([3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], ObjectFile.Empty(Kind.Buff).ToBytes());
    }

    [Fact]
    public void Version_is_the_files()
    {
        var bytes = Fixtures.Bytes("we-3.0/war3map.w3u");
        byte[] version2 = [2, 0, 0, 0, .. bytes[4..20], .. bytes[28..]];   // without h000's set count and flag

        Assert.Equal(3, Fixtures.Read("we-3.0/war3map.w3u").Version);
        Assert.Equal(3, ObjectFile.Empty(Kind.Ability).Version);
        var older = ObjectFile.ReadExact(Kind.Unit, version2);
        Assert.Equal(2, older.Version);
        Assert.Equal(0, older.Custom[0].SetCount);
    }

    [Fact]
    public void Original_entry_has_no_id()
    {
        var units = ObjectFile.Empty(Kind.Unit);

        var footman = units.AddOriginal("hfoo");
        footman.Set("uhpm", 0, 0, new FieldValue.Int(500));

        var again = ObjectFile.ReadExact(Kind.Unit, units.ToBytes());
        var entry = Assert.Single(again.Original);
        Assert.Equal("hfoo", entry.BaseId);
        Assert.Equal("", entry.Id);
        Assert.Equal(1, entry.SetCount);
        Assert.Equal(new FieldValue.Int(500), entry.Find("uhpm", 0)!.Value);
    }

    [Fact]
    public void Kind_without_levels_refuses_a_level()
    {
        var units = ObjectFile.Empty(Kind.Unit);
        var entry = units.AddCustom("hpea", "h000");

        Assert.Throws<ArgumentException>(() => entry.Set("uhpm", 1, 0, new FieldValue.Int(1)));
        Assert.Throws<ArgumentException>(() => units.AddCustom("hpea", "h0"));
    }

    [Fact]
    public void Skin_split_matches_fixture()
    {
        var where = new Dictionary<string, SortedSet<string>>();
        foreach (var folder in Fixtures.Folders)
        {
            var files = Fixtures.ObjectFiles().Where(file => file.StartsWith(folder + "/", StringComparison.Ordinal));
            foreach (var file in files)
            {
                var side = Path.GetFileName(file).StartsWith("war3mapSkin", StringComparison.Ordinal) ? "skin" : "main";
                var objectFile = Fixtures.Read(file);
                foreach (var entry in objectFile.Original.Concat(objectFile.Custom))
                {
                    foreach (var mod in entry.Mods)
                    {
                        var id = entry.Id == "" ? entry.BaseId : entry.Id;
                        output.WriteLine($"{file} | {id} | {mod.Field} | {mod.Level} | {mod.Pointer} | {side}");
                        if (folder == "we-3.0")
                        {
                            if (!where.TryGetValue(mod.Field, out var sides))
                            {
                                sides = [];
                                where[mod.Field] = sides;
                            }

                            sides.Add(side);
                        }
                    }
                }
            }
        }

        Assert.Equal(["anam", "bnam", "dnam", "fnam", "gnam", "unam"], where.Keys.Order(StringComparer.Ordinal));
        foreach (var field in new[] { "unam", "anam", "gnam", "fnam", "bnam" })
        {
            Assert.Equal(["skin"], where[field]);
        }

        Assert.Equal(["main"], where["dnam"]);
    }

    // A unit file with one new unit holding a Bool and a Char modification, written by War3Net.
    private static byte[] BoolAndCharUnitFile()
    {
        var data = new UnitObjectData(ObjectDataFormatVersion.v3);
#pragma warning disable CS0612 // War3Net marks its Bool and Char value types [Obsolete]; files still hold them.
        data.NewUnits.Add(new SimpleObjectModification
        {
            OldId = Rawcodes.IdOf("hpea"),
            NewId = Rawcodes.IdOf("h000"),
            Unk = { 0 },
            Modifications =
            {
                new SimpleObjectDataModification
                {
                    Id = Rawcodes.IdOf("ubol"),
                    Type = ObjectDataType.Bool,
                    Value = true,
                },
                new SimpleObjectDataModification
                {
                    Id = Rawcodes.IdOf("uchr"),
                    Type = ObjectDataType.Char,
                    Value = 'x',
                },
            },
        });
#pragma warning restore CS0612

        using var stream = new MemoryStream();
        using (var writer = new BinaryWriter(stream))
        {
            writer.Write(data);
        }

        return stream.ToArray();
    }

    private static List<string> Snapshot(ObjectFile file)
    {
        var lines = new List<string>();
        foreach (var entry in file.Original.Concat(file.Custom))
        {
            var id = entry.Id == "" ? entry.BaseId : entry.Id;
            foreach (var mod in entry.Mods)
            {
                lines.Add($"{id} {mod.Field} {mod.Level} {mod.Pointer} {mod.Value}");
            }
        }

        return lines;
    }

    private static int IndexOf(IReadOnlyList<FieldMod> mods, FieldMod mod)
    {
        for (var i = 0; i < mods.Count; i++)
        {
            if (ReferenceEquals(mods[i], mod))
            {
                return i;
            }
        }

        return -1;
    }

    private static int IndexOf(byte[] bytes, ReadOnlySpan<byte> part) => bytes.AsSpan().IndexOf(part);
}
