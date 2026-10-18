using System.Buffers.Binary;
using System.Text;
using War3Net.Build.Extensions;
using War3Net.Build.Object;

namespace Moonwell.Build
{
    // One object-data file as the build edits it: the objects of the original table (a standard object changed) and of the
    // new table (an object based on another), each with its modifications. It keeps what the file holds, so that a file
    // read and written again is the same bytes. The seven kinds differ in War3Net's types only: units, items, destructables
    // and buffs have plain modifications, abilities and upgrades add a level and a data pointer, doodads a variation and a pointer.
    internal sealed class ObjectTable
    {
        private const ObjectDataFormatVersion NewFileVersion = ObjectDataFormatVersion.v3;

        private ObjectTable(ObjectDataFormatVersion version, bool existed)
        {
            Version = version;
            Existed = existed;
        }

        /// <summary>One modification: a field's value, for a level (or variation) and data pointer where the kind has them.</summary>
        public sealed record Modification(string Field, ObjectDataType Type, object Value, int Level, int Pointer);

        /// <summary>One object with its modifications.</summary>
        public sealed class Entry
        {
            public Entry(string oldId, string newId, List<int> unknown, List<Modification> modifications)
            {
                OldId = oldId;
                NewId = newId;
                Unknown = unknown;
                Modifications = modifications;
            }

            /// <summary>The id of the standard object it changes or is based on.</summary>
            public string OldId { get; }

            /// <summary>The object's own id; empty in the original table.</summary>
            public string NewId { get; }

            /// <summary>What format 3 puts before the modifications: the number of sets and each set's flags.</summary>
            public List<int> Unknown { get; }

            public List<Modification> Modifications { get; }

            /// <summary>The id the object has: its own, else the standard object's.</summary>
            public string Id => NewId.Length > 0 ? NewId : OldId;
        }

        /// <summary>Whether the file existed when it was read.</summary>
        public bool Existed { get; }

        public ObjectDataFormatVersion Version { get; }

        /// <summary>The standard objects the file changes.</summary>
        public List<Entry> Originals { get; } = new List<Entry>();

        /// <summary>The objects the file adds.</summary>
        public List<Entry> News { get; } = new List<Entry>();

        /// <summary>Whether the file would hold any object.</summary>
        public bool IsEmpty => Originals.Count == 0 && News.Count == 0;

        /// <summary>Reads an object-data file; a missing file is an empty format 3 table.</summary>
        /// <param name="path">The file.</param>
        /// <param name="kind">The kind the file holds: <c>unit</c>, <c>item</c>, <c>destructable</c>, <c>doodad</c>, <c>ability</c>, <c>buff</c> or <c>upgrade</c>.</param>
        public static ObjectTable Read(string path, string kind)
        {
            if (!File.Exists(path))
            {
                return new ObjectTable(NewFileVersion, false);
            }

            try
            {
                using var stream = File.OpenRead(path);
                using var reader = new BinaryReader(stream, Encoding.UTF8);
                switch (kind)
                {
                    case "unit":
                        var units = reader.ReadUnitObjectData();
                        return FromSimple(units.FormatVersion, units.BaseUnits, units.NewUnits);
                    case "item":
                        var items = reader.ReadItemObjectData();
                        return FromSimple(items.FormatVersion, items.BaseItems, items.NewItems);
                    case "destructable":
                        var destructables = reader.ReadDestructableObjectData();
                        return FromSimple(destructables.FormatVersion, destructables.BaseDestructables, destructables.NewDestructables);
                    case "buff":
                        var buffs = reader.ReadBuffObjectData();
                        return FromSimple(buffs.FormatVersion, buffs.BaseBuffs, buffs.NewBuffs);
                    case "ability":
                        var abilities = reader.ReadAbilityObjectData();
                        return FromLevel(abilities.FormatVersion, abilities.BaseAbilities, abilities.NewAbilities);
                    case "upgrade":
                        var upgrades = reader.ReadUpgradeObjectData();
                        return FromLevel(upgrades.FormatVersion, upgrades.BaseUpgrades, upgrades.NewUpgrades);
                    case "doodad":
                        var doodads = reader.ReadDoodadObjectData();
                        return FromVariation(doodads.FormatVersion, doodads.BaseDoodads, doodads.NewDoodads);
                    default:
                        throw new ArgumentException($"no object kind {kind}", nameof(kind));
                }
            }
            catch (Exception failure) when (failure is EndOfStreamException or IOException or InvalidDataException or NotSupportedException or ArgumentOutOfRangeException)
            {
                throw new BuildException($"cannot read {Path.GetFileName(path)}: {failure.Message}");
            }
        }

        /// <summary>Gives an entry the format's set list for a new object: one set, no flags.</summary>
        public List<int> NewUnknown()
        {
            return Version >= ObjectDataFormatVersion.v3 ? new List<int> { 0 } : new List<int>();
        }

        /// <summary>Writes the table as an object-data file.</summary>
        /// <param name="path">The file; its folder exists.</param>
        /// <param name="kind">The kind the file holds, as for <see cref="Read"/>.</param>
        public void Write(string path, string kind)
        {
            using var stream = File.Create(path);
            using var writer = new BinaryWriter(stream, Encoding.UTF8);
            switch (kind)
            {
                case "unit":
                    writer.Write(new UnitObjectData(Version) { BaseUnits = ToSimple(Originals), NewUnits = ToSimple(News) });
                    break;
                case "item":
                    writer.Write(new ItemObjectData(Version) { BaseItems = ToSimple(Originals), NewItems = ToSimple(News) });
                    break;
                case "destructable":
                    writer.Write(new DestructableObjectData(Version) { BaseDestructables = ToSimple(Originals), NewDestructables = ToSimple(News) });
                    break;
                case "buff":
                    writer.Write(new BuffObjectData(Version) { BaseBuffs = ToSimple(Originals), NewBuffs = ToSimple(News) });
                    break;
                case "ability":
                    writer.Write(new AbilityObjectData(Version) { BaseAbilities = ToLevel(Originals), NewAbilities = ToLevel(News) });
                    break;
                case "upgrade":
                    writer.Write(new UpgradeObjectData(Version) { BaseUpgrades = ToLevel(Originals), NewUpgrades = ToLevel(News) });
                    break;
                case "doodad":
                    writer.Write(new DoodadObjectData(Version) { BaseDoodads = ToVariation(Originals), NewDoodads = ToVariation(News) });
                    break;
                default:
                    throw new ArgumentException($"no object kind {kind}", nameof(kind));
            }
        }

        /// <summary>The four characters an id integer holds.</summary>
        public static string TextOf(int id)
        {
            var bytes = new byte[4];
            BinaryPrimitives.WriteInt32LittleEndian(bytes, id);
            return Encoding.ASCII.GetString(bytes).TrimEnd('\0');
        }

        /// <summary>The integer of an id's four characters; an empty text is 0.</summary>
        public static int IdOf(string text)
        {
            var bytes = new byte[4];
            Encoding.ASCII.GetBytes(text, 0, Math.Min(4, text.Length), bytes, 0);
            return BinaryPrimitives.ReadInt32LittleEndian(bytes);
        }

        private static ObjectTable FromSimple(ObjectDataFormatVersion version, List<SimpleObjectModification> originals, List<SimpleObjectModification> news)
        {
            var table = new ObjectTable(version, true);
            foreach (var one in originals)
            {
                table.Originals.Add(EntryOfSimple(one));
            }

            foreach (var one in news)
            {
                table.News.Add(EntryOfSimple(one));
            }

            return table;
        }

        private static ObjectTable FromLevel(ObjectDataFormatVersion version, List<LevelObjectModification> originals, List<LevelObjectModification> news)
        {
            var table = new ObjectTable(version, true);
            foreach (var one in originals)
            {
                table.Originals.Add(EntryOfLevel(one));
            }

            foreach (var one in news)
            {
                table.News.Add(EntryOfLevel(one));
            }

            return table;
        }

        private static ObjectTable FromVariation(ObjectDataFormatVersion version, List<VariationObjectModification> originals, List<VariationObjectModification> news)
        {
            var table = new ObjectTable(version, true);
            foreach (var one in originals)
            {
                table.Originals.Add(EntryOfVariation(one));
            }

            foreach (var one in news)
            {
                table.News.Add(EntryOfVariation(one));
            }

            return table;
        }

        private static Entry EntryOfSimple(SimpleObjectModification one)
        {
            var mods = one.Modifications.Select(mod => new Modification(TextOf(mod.Id), mod.Type, mod.Value, 0, 0)).ToList();
            return new Entry(TextOf(one.OldId), TextOf(one.NewId), new List<int>(one.Unk), mods);
        }

        private static Entry EntryOfLevel(LevelObjectModification one)
        {
            var mods = one.Modifications.Select(mod => new Modification(TextOf(mod.Id), mod.Type, mod.Value, mod.Level, mod.Pointer)).ToList();
            return new Entry(TextOf(one.OldId), TextOf(one.NewId), new List<int>(one.Unk), mods);
        }

        private static Entry EntryOfVariation(VariationObjectModification one)
        {
            var mods = one.Modifications.Select(mod => new Modification(TextOf(mod.Id), mod.Type, mod.Value, mod.Variation, mod.Pointer)).ToList();
            return new Entry(TextOf(one.OldId), TextOf(one.NewId), new List<int>(one.Unk), mods);
        }

        private static List<SimpleObjectModification> ToSimple(List<Entry> entries)
        {
            return entries.Select(entry => new SimpleObjectModification
            {
                OldId = IdOf(entry.OldId),
                NewId = IdOf(entry.NewId),
                Unk = new List<int>(entry.Unknown),
                Modifications = entry.Modifications
                    .Select(mod => new SimpleObjectDataModification { Id = IdOf(mod.Field), Type = mod.Type, Value = mod.Value })
                    .ToList(),
            }).ToList();
        }

        private static List<LevelObjectModification> ToLevel(List<Entry> entries)
        {
            return entries.Select(entry => new LevelObjectModification
            {
                OldId = IdOf(entry.OldId),
                NewId = IdOf(entry.NewId),
                Unk = new List<int>(entry.Unknown),
                Modifications = entry.Modifications
                    .Select(mod => new LevelObjectDataModification { Id = IdOf(mod.Field), Type = mod.Type, Value = mod.Value, Level = mod.Level, Pointer = mod.Pointer })
                    .ToList(),
            }).ToList();
        }

        private static List<VariationObjectModification> ToVariation(List<Entry> entries)
        {
            return entries.Select(entry => new VariationObjectModification
            {
                OldId = IdOf(entry.OldId),
                NewId = IdOf(entry.NewId),
                Unk = new List<int>(entry.Unknown),
                Modifications = entry.Modifications
                    .Select(mod => new VariationObjectDataModification { Id = IdOf(mod.Field), Type = mod.Type, Value = mod.Value, Variation = mod.Level, Pointer = mod.Pointer })
                    .ToList(),
            }).ToList();
        }
    }
}
