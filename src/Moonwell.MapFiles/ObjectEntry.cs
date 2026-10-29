using Moonwell.GameData;
using War3Net.Build.Object;
using War3Net.Common.Extensions;

namespace Moonwell.MapFiles;

/// <summary>
/// One object's record in an object file: a change to a standard object, or a custom object. It wraps War3Net's record
/// and edits it in place.
/// </summary>
public sealed class ObjectEntry
{
    // War3Net has one record class per modification layout; exactly one of these is set.
    private readonly SimpleObjectModification? simple;
    private readonly LevelObjectModification? levels;
    private readonly VariationObjectModification? variations;
    private readonly List<FieldMod> mods = [];

    internal ObjectEntry(object record)
    {
        Record = record;
        switch (record)
        {
            case SimpleObjectModification simpleRecord:
                simple = simpleRecord;
                foreach (var mod in simpleRecord.Modifications)
                {
                    mods.Add(new FieldMod(mod));
                }

                break;
            case LevelObjectModification levelRecord:
                levels = levelRecord;
                foreach (var mod in levelRecord.Modifications)
                {
                    mods.Add(new FieldMod(mod));
                }

                break;
            case VariationObjectModification variationRecord:
                variations = variationRecord;
                foreach (var mod in variationRecord.Modifications)
                {
                    mods.Add(new FieldMod(mod));
                }

                break;
            default:
                throw new ArgumentException("Not a War3Net object record.", nameof(record));
        }
    }

    /// <summary>War3Net's record this wraps.</summary>
    internal object Record { get; }

    /// <summary>
    /// The standard object's id: for a custom object the one it is based on, for a change to a standard object that
    /// object itself. Always four characters, as War3Net's <c>ToRawcode</c> gives them, a NUL byte included (unlike
    /// <c>Rawcodes.TextOf</c>, which trims trailing NULs).
    /// </summary>
    public string BaseId
    {
        get => OldId.ToRawcode();
        set => OldId = Rawcodes.From(value, nameof(BaseId));
    }

    /// <summary>
    /// The custom object's id; "" for a change to a standard object. Otherwise four characters, as War3Net's
    /// <c>ToRawcode</c> gives them, a NUL byte included (unlike <c>Rawcodes.TextOf</c>, which trims trailing NULs).
    /// </summary>
    public string Id
    {
        get => NewId == 0 ? "" : NewId.ToRawcode();
        set => NewId = value == "" ? 0 : Rawcodes.From(value, nameof(Id));
    }

    /// <summary>
    /// How many set flags the record has (version 3 files; World Editor writes one, 0). Always 0 in older versions,
    /// which have none.
    /// </summary>
    public int SetCount => Flags.Count;

    /// <summary>
    /// The record's set flags (what War3Net calls <c>Unk</c>), so a reader sees them besides <see cref="SetCount"/>:
    /// empty in versions before 3, which have none.
    /// </summary>
    public IReadOnlyList<int> SetFlags => Flags;

    /// <summary>The field modifications, in file order.</summary>
    public IReadOnlyList<FieldMod> Mods => mods;

    /// <summary>The modification of this field at this level (0 for kinds without levels), or null.</summary>
    public FieldMod? Find(string field, int level) =>
        mods.FirstOrDefault(mod => mod.Field == field && mod.Level == level);

    /// <summary>
    /// Sets a field at a level: changes the modification already there in place (keeping its position), or appends a
    /// new one. Level and pointer must be 0 for kinds without levels.
    /// </summary>
    public FieldMod Set(string field, int level, int pointer, FieldValue value)
    {
        var id = Rawcodes.From(field, nameof(field));
        ArgumentNullException.ThrowIfNull(value);
        if (simple is not null && (level != 0 || pointer != 0))
        {
            throw new ArgumentException("Level and pointer must be 0 for a kind without levels.", nameof(level));
        }

        var existing = Find(field, level);
        if (existing is not null)
        {
            existing.Value = value;
            existing.Pointer = pointer;
            return existing;
        }

        // The new modification starts as an empty text, so a refused value leaves nothing behind.
        ObjectDataModification record = simple is not null
            ? new SimpleObjectDataModification()
            : levels is not null
                ? new LevelObjectDataModification()
                : new VariationObjectDataModification();
        record.Id = id;
        record.Type = ObjectDataType.String;
        record.Value = "";
        var mod = new FieldMod(record) { Value = value, Level = level, Pointer = pointer };

        switch (record)
        {
            case SimpleObjectDataModification simpleMod:
                simple!.Modifications.Add(simpleMod);
                break;
            case LevelObjectDataModification levelMod:
                levels!.Modifications.Add(levelMod);
                break;
            case VariationObjectDataModification variationMod:
                variations!.Modifications.Add(variationMod);
                break;
        }

        mods.Add(mod);
        return mod;
    }

    /// <summary>
    /// Puts a modification that was removed from this entry back at <paramref name="index"/> of <see cref="Mods"/>. An
    /// undo uses it, so the record is as it was before; another modification of the same field and level is left as
    /// it is.
    /// </summary>
    public void Insert(int index, FieldMod mod)
    {
        ArgumentNullException.ThrowIfNull(mod);
        if (mods.Contains(mod))
        {
            throw new ArgumentException("The modification is already in this entry.", nameof(mod));
        }

        var fits = mod.Record switch
        {
            SimpleObjectDataModification => simple is not null,
            LevelObjectDataModification => levels is not null,
            VariationObjectDataModification => variations is not null,
            _ => false,
        };
        if (!fits)
        {
            throw new ArgumentException("The modification is not of this entry's layout.", nameof(mod));
        }

        ArgumentOutOfRangeException.ThrowIfNegative(index);
        ArgumentOutOfRangeException.ThrowIfGreaterThan(index, mods.Count);

        // War3Net's list holds the same modifications in the same order, so the index is the same in both.
        switch (mod.Record)
        {
            case SimpleObjectDataModification simpleMod:
                simple!.Modifications.Insert(index, simpleMod);
                break;
            case LevelObjectDataModification levelMod:
                levels!.Modifications.Insert(index, levelMod);
                break;
            case VariationObjectDataModification variationMod:
                variations!.Modifications.Insert(index, variationMod);
                break;
        }

        mods.Insert(index, mod);
    }

    /// <summary>Removes a modification of this entry.</summary>
    public void Remove(FieldMod mod)
    {
        if (!mods.Remove(mod))
        {
            throw new ArgumentException("The modification is not in this entry.", nameof(mod));
        }

        switch (mod.Record)
        {
            case SimpleObjectDataModification simpleMod:
                simple!.Modifications.Remove(simpleMod);
                break;
            case LevelObjectDataModification levelMod:
                levels!.Modifications.Remove(levelMod);
                break;
            case VariationObjectDataModification variationMod:
                variations!.Modifications.Remove(variationMod);
                break;
        }
    }

    /// <summary>A new record of the layout the kind uses.</summary>
    internal static ObjectEntry Create(Kind kind, int oldId, int newId, bool withSetFlag)
    {
        List<int> flags = withSetFlag ? [0] : [];
        object record = kind switch
        {
            Kind.Doodad => new VariationObjectModification { OldId = oldId, NewId = newId, Unk = flags },
            Kind.Ability or Kind.Upgrade =>
                new LevelObjectModification { OldId = oldId, NewId = newId, Unk = flags },
            _ => new SimpleObjectModification { OldId = oldId, NewId = newId, Unk = flags },
        };
        return new ObjectEntry(record);
    }

    private int OldId
    {
        get => simple?.OldId ?? levels?.OldId ?? variations!.OldId;
        set
        {
            if (simple is not null)
            {
                simple.OldId = value;
            }
            else if (levels is not null)
            {
                levels.OldId = value;
            }
            else
            {
                variations!.OldId = value;
            }
        }
    }

    private int NewId
    {
        get => simple?.NewId ?? levels?.NewId ?? variations!.NewId;
        set
        {
            if (simple is not null)
            {
                simple.NewId = value;
            }
            else if (levels is not null)
            {
                levels.NewId = value;
            }
            else
            {
                variations!.NewId = value;
            }
        }
    }

    // War3Net calls the set flags Unk.
    private List<int> Flags => simple?.Unk ?? levels?.Unk ?? variations!.Unk;
}
