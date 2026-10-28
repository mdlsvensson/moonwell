using System.Collections;
using System.Diagnostics;
using Moonwell.GameData;
using War3Net.Build.Extensions;
using War3Net.Build.Object;

namespace Moonwell.MapFiles;

/// <summary>
/// One object file of one kind (the main <c>war3map.w3u</c> or the skin <c>war3mapSkin.w3u</c>), read and written by
/// War3Net. The entries wrap War3Net's records and edit them in place.
/// </summary>
public sealed class ObjectFile
{
    // War3Net's object, one of its seven object data classes, written back by ToBytes.
    private readonly object data;

    // War3Net's two tables, as the non-generic view of its typed lists, so one Add and Remove serve every kind.
    private readonly IList originalRecords;
    private readonly IList customRecords;

    private readonly List<ObjectEntry> original = [];
    private readonly List<ObjectEntry> custom = [];

    private ObjectFile(Kind kind, object data, IList originalRecords, IList customRecords)
    {
        Kind = kind;
        this.data = data;
        this.originalRecords = originalRecords;
        this.customRecords = customRecords;
        foreach (var record in originalRecords)
        {
            original.Add(new ObjectEntry(record));
        }

        foreach (var record in customRecords)
        {
            custom.Add(new ObjectEntry(record));
        }
    }

    /// <summary>The kind of object the file holds.</summary>
    public Kind Kind { get; }

    /// <summary>The changes to standard objects.</summary>
    public IReadOnlyList<ObjectEntry> Original => original;

    /// <summary>The custom objects.</summary>
    public IReadOnlyList<ObjectEntry> Custom => custom;

    /// <summary>
    /// Reads an object file with War3Net and nothing more (the lenient read): whatever War3Net reads is kept, even a
    /// bool or char value (<see cref="FieldValue.Bool"/>, <see cref="FieldValue.Char"/>) and bytes it would not write
    /// back. Throws an <see cref="ObjectFileException"/> ("War3Net cannot read ...") when War3Net cannot read the file.
    /// <paramref name="name"/> is the file's name in messages (by default <c>war3map.</c> and the kind's extension).
    /// </summary>
    public static ObjectFile Read(Kind kind, byte[] bytes, string? name = null)
    {
        ArgumentNullException.ThrowIfNull(bytes);
        var extension = Kinds.Extension(kind);
        name ??= "war3map." + extension;

        try
        {
            using var stream = new MemoryStream(bytes, false);
            using var reader = new BinaryReader(stream);
            return Wrap(kind, kind switch
            {
                Kind.Unit => reader.ReadUnitObjectData(),
                Kind.Item => reader.ReadItemObjectData(),
                Kind.Destructable => reader.ReadDestructableObjectData(),
                Kind.Doodad => reader.ReadDoodadObjectData(),
                Kind.Ability => reader.ReadAbilityObjectData(),
                Kind.Buff => reader.ReadBuffObjectData(),
                Kind.Upgrade => reader.ReadUpgradeObjectData(),
                _ => throw new UnreachableException(),   // Kinds.Extension checked the kind
            });
        }
        catch (Exception e)
        {
            // War3Net's own exceptions: the end of the stream, a string with no end, an unknown value type.
            throw new ObjectFileException($"War3Net cannot read {name}: {e.Message}", e);
        }
    }

    /// <summary>
    /// Reads an object file with War3Net (<see cref="Read(Kind, byte[], string?)"/>), then refuses a file the editor
    /// cannot save exactly: one with a bool or char value, and one War3Net does not write back to the same bytes. Throws
    /// an <see cref="ObjectFileException"/> naming the problem; for a file War3Net cannot write back exactly the
    /// exception's <see cref="ObjectFileException.Parsed"/> holds what War3Net read, to show but never to save.
    /// </summary>
    public static ObjectFile ReadExact(Kind kind, byte[] bytes, string? name = null) =>
        ReadExact(kind, bytes, name, file => file.ToBytes());

    /// <summary>
    /// <see cref="ReadExact(Kind, byte[], string?)"/> with the guard's write-back given. War3Net (from 6.1.2-moonwell.4)
    /// writes every known file back exactly, so tests pass a <paramref name="writeBack"/> that differs to stand for a
    /// file it cannot.
    /// </summary>
    internal static ObjectFile ReadExact(Kind kind, byte[] bytes, string? name, Func<ObjectFile, byte[]> writeBack)
    {
        var file = Read(kind, bytes, name);
        name ??= "war3map." + Kinds.Extension(kind);

        foreach (var entry in file.original.Concat(file.custom))
        {
            var odd = entry.Mods.FirstOrDefault(mod => !mod.HasKnownType);
            if (odd is not null)
            {
                throw new ObjectFileException($"{name} has a modification of {odd.Field} with value type "
                    + $"{(int)odd.Record.Type}, which the editor does not support.");
            }
        }

        file.CheckWritesBack(bytes, name, writeBack);
        return file;
    }

    /// <summary>An empty file of the kind, in version 3 (since patch 1.33), with no records.</summary>
    public static ObjectFile Empty(Kind kind)
    {
        const ObjectDataFormatVersion version = ObjectDataFormatVersion.v3;
        return Wrap(kind, kind switch
        {
            Kind.Unit => new UnitObjectData(version),
            Kind.Item => new ItemObjectData(version),
            Kind.Destructable => new DestructableObjectData(version),
            Kind.Doodad => new DoodadObjectData(version),
            Kind.Ability => new AbilityObjectData(version),
            Kind.Buff => new BuffObjectData(version),
            Kind.Upgrade => new UpgradeObjectData(version),
            _ => throw new ArgumentOutOfRangeException(nameof(kind), kind, null),
        });
    }

    /// <summary>
    /// Adds a custom object based on a standard one, with no modifications (set flags [0] in version 3).
    /// </summary>
    public ObjectEntry AddCustom(string baseId, string id)
    {
        var entry = ObjectEntry.Create(
            Kind, Rawcodes.From(baseId, nameof(baseId)), Rawcodes.From(id, nameof(id)), HasSetFlags);
        customRecords.Add(entry.Record);
        custom.Add(entry);
        return entry;
    }

    /// <summary>Adds a change to a standard object, with no modifications (set flags [0] in version 3).</summary>
    public ObjectEntry AddOriginal(string id)
    {
        var entry = ObjectEntry.Create(Kind, Rawcodes.From(id, nameof(id)), 0, HasSetFlags);
        originalRecords.Add(entry.Record);
        original.Add(entry);
        return entry;
    }

    /// <summary>Removes an entry from whichever table holds it.</summary>
    public void Remove(ObjectEntry entry)
    {
        if (original.Remove(entry))
        {
            originalRecords.Remove(entry.Record);
        }
        else if (custom.Remove(entry))
        {
            customRecords.Remove(entry.Record);
        }
        else
        {
            throw new ArgumentException("The entry is not in this file.", nameof(entry));
        }
    }

    /// <summary>
    /// Puts an entry that was removed from this file back at <paramref name="index"/> of its table: the custom objects
    /// when it has an id, else the changes to standard objects. An undo uses it, so the file is as it was before.
    /// </summary>
    public void Insert(int index, ObjectEntry entry)
    {
        ArgumentNullException.ThrowIfNull(entry);
        if (original.Contains(entry) || custom.Contains(entry))
        {
            throw new ArgumentException("The entry is already in this file.", nameof(entry));
        }

        var fits = Kind switch
        {
            Kind.Doodad => entry.Record is VariationObjectModification,
            Kind.Ability or Kind.Upgrade => entry.Record is LevelObjectModification,
            _ => entry.Record is SimpleObjectModification,
        };
        if (!fits)
        {
            throw new ArgumentException($"The entry is not of a {Kinds.Key(Kind)} file.", nameof(entry));
        }

        var (entries, records) = entry.Id == "" ? (original, originalRecords) : (custom, customRecords);
        ArgumentOutOfRangeException.ThrowIfNegative(index);
        ArgumentOutOfRangeException.ThrowIfGreaterThan(index, entries.Count);
        records.Insert(index, entry.Record);
        entries.Insert(index, entry);
    }

    /// <summary>The file's bytes, as War3Net writes them.</summary>
    public byte[] ToBytes()
    {
        using var stream = new MemoryStream();
        using (var writer = new BinaryWriter(stream))
        {
            switch (data)
            {
                case UnitObjectData units:
                    writer.Write(units);
                    break;
                case ItemObjectData items:
                    writer.Write(items);
                    break;
                case DestructableObjectData destructables:
                    writer.Write(destructables);
                    break;
                case DoodadObjectData doodads:
                    writer.Write(doodads);
                    break;
                case AbilityObjectData abilities:
                    writer.Write(abilities);
                    break;
                case BuffObjectData buffs:
                    writer.Write(buffs);
                    break;
                case UpgradeObjectData upgrades:
                    writer.Write(upgrades);
                    break;
            }
        }

        return stream.ToArray();
    }

    /// <summary>
    /// The file's format version: 1, 2 or 3 (since patch 1.33). Only version 3 records carry set flags.
    /// </summary>
    public int Version => (int)FormatVersion;

    // Records carry set flags since version 3.
    private bool HasSetFlags => FormatVersion >= ObjectDataFormatVersion.v3;

    private ObjectDataFormatVersion FormatVersion => data switch
    {
        UnitObjectData units => units.FormatVersion,
        ItemObjectData items => items.FormatVersion,
        DestructableObjectData destructables => destructables.FormatVersion,
        DoodadObjectData doodads => doodads.FormatVersion,
        AbilityObjectData abilities => abilities.FormatVersion,
        BuffObjectData buffs => buffs.FormatVersion,
        UpgradeObjectData upgrades => upgrades.FormatVersion,
        _ => throw new InvalidOperationException("Not a War3Net object file."),
    };

    private static ObjectFile Wrap(Kind kind, object data) => data switch
    {
        UnitObjectData units => new ObjectFile(kind, data, units.BaseUnits, units.NewUnits),
        ItemObjectData items => new ObjectFile(kind, data, items.BaseItems, items.NewItems),
        DestructableObjectData destructables =>
            new ObjectFile(kind, data, destructables.BaseDestructables, destructables.NewDestructables),
        DoodadObjectData doodads => new ObjectFile(kind, data, doodads.BaseDoodads, doodads.NewDoodads),
        AbilityObjectData abilities => new ObjectFile(kind, data, abilities.BaseAbilities, abilities.NewAbilities),
        BuffObjectData buffs => new ObjectFile(kind, data, buffs.BaseBuffs, buffs.NewBuffs),
        UpgradeObjectData upgrades => new ObjectFile(kind, data, upgrades.BaseUpgrades, upgrades.NewUpgrades),
        _ => throw new ArgumentException("Not a War3Net object file.", nameof(data)),
    };

    /// <summary>
    /// The round-trip guard: what War3Net read must write back to the same bytes, or the file is never saved through
    /// this read.
    /// </summary>
    private void CheckWritesBack(byte[] bytes, string name, Func<ObjectFile, byte[]> writeBack)
    {
        byte[] written;
        try
        {
            written = writeBack(this);
        }
        catch (Exception e)
        {
            throw new ObjectFileException($"War3Net cannot write {name} back: {e.Message}", e, this);
        }

        if (written.AsSpan().SequenceEqual(bytes))
        {
            return;
        }

        var at = written.AsSpan().CommonPrefixLength(bytes);
        var detail = at == written.Length
            ? $"it would drop the last {bytes.Length - written.Length} bytes, from offset {at}"
            : $"the bytes differ from offset {at} ({written.Length} bytes written, {bytes.Length} read)";
        throw new ObjectFileException($"War3Net cannot write {name} back exactly: {detail}.", null, this);
    }
}

/// <summary>
/// An object file War3Net cannot read, or cannot write back exactly. The message names the file and the problem.
/// </summary>
public sealed class ObjectFileException : Exception
{
    /// <summary>A problem with an object file.</summary>
    public ObjectFileException(string message)
        : base(message)
    {
    }

    /// <summary>A problem with an object file, caused by War3Net's exception.</summary>
    public ObjectFileException(string message, Exception inner)
        : base(message, inner)
    {
    }

    /// <summary>
    /// A file War3Net read but cannot write back exactly: <paramref name="parsed"/> is what it read.
    /// </summary>
    internal ObjectFileException(string message, Exception? inner, ObjectFile parsed)
        : base(message, inner)
    {
        Parsed = parsed;
    }

    /// <summary>
    /// What War3Net read when it read the file but cannot write it back exactly, so its records can still be shown;
    /// it must never be saved. Null when War3Net could not read the file at all.
    /// </summary>
    public ObjectFile? Parsed { get; }
}
