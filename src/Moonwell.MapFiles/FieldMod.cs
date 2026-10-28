using System.Text;
using Moonwell.GameData;
using War3Net.Build.Object;
using War3Net.Common.Extensions;

namespace Moonwell.MapFiles;

/// <summary>
/// One modification of one field of one object, as an object file stores it. It wraps War3Net's modification and
/// edits it in place, so whatever War3Net read and this class does not show (the end marker) is written back as read.
/// </summary>
public sealed class FieldMod
{
    private static readonly UTF8Encoding StrictUtf8 = new(false, true);

    internal FieldMod(ObjectDataModification record)
    {
        Record = record;
    }

    /// <summary>War3Net's modification this wraps.</summary>
    internal ObjectDataModification Record { get; }

    /// <summary>The field's four-character id, such as "uhpm".</summary>
    public string Field
    {
        get => Record.Id.ToRawcode();
        set => Record.Id = Rawcodes.From(value, nameof(Field));
    }

    /// <summary>The level (the variation for doodads); always 0 for kinds without levels.</summary>
    public int Level
    {
        get => Record switch
        {
            LevelObjectDataModification level => level.Level,
            VariationObjectDataModification variation => variation.Variation,
            _ => 0,
        };
        set
        {
            switch (Record)
            {
                case LevelObjectDataModification level:
                    level.Level = value;
                    break;
                case VariationObjectDataModification variation:
                    variation.Variation = value;
                    break;
                default:
                    RequireZero(value, nameof(Level));
                    break;
            }
        }
    }

    /// <summary>
    /// The data pointer (which of an ability's Data A, B, C ... fields); always 0 for kinds without levels.
    /// </summary>
    public int Pointer
    {
        get => Record switch
        {
            LevelObjectDataModification level => level.Pointer,
            VariationObjectDataModification variation => variation.Pointer,
            _ => 0,
        };
        set
        {
            switch (Record)
            {
                case LevelObjectDataModification level:
                    level.Pointer = value;
                    break;
                case VariationObjectDataModification variation:
                    variation.Pointer = value;
                    break;
                default:
                    RequireZero(value, nameof(Pointer));
                    break;
            }
        }
    }

    /// <summary>The value, with its type.</summary>
    public FieldValue Value
    {
        get => Record.Type switch
        {
            ObjectDataType.Int => new FieldValue.Int((int)Record.Value),
            ObjectDataType.Real => new FieldValue.Real((float)Record.Value),
            ObjectDataType.Unreal => new FieldValue.Unreal((float)Record.Value),
            ObjectDataType.String => new FieldValue.Text((string)Record.Value),
#pragma warning disable CS0612 // War3Net marks its Bool and Char value types [Obsolete]; files still hold them.
            ObjectDataType.Bool => new FieldValue.Bool((bool)Record.Value),
            ObjectDataType.Char => new FieldValue.Char((char)Record.Value),
#pragma warning restore CS0612
            _ => throw new InvalidOperationException($"Value type {(int)Record.Type} is not supported."),
        };
        set
        {
            switch (value)
            {
                case FieldValue.Int number:
                    Record.Type = ObjectDataType.Int;
                    Record.Value = number.Value;
                    break;
                case FieldValue.Real real:
                    Record.Type = ObjectDataType.Real;
                    Record.Value = real.Value;
                    break;
                case FieldValue.Unreal unreal:
                    Record.Type = ObjectDataType.Unreal;
                    Record.Value = unreal.Value;
                    break;
                case FieldValue.Text text:
                    CheckText(text.Value);
                    Record.Type = ObjectDataType.String;
                    Record.Value = text.Value;
                    break;
#pragma warning disable CS0612 // War3Net marks its Bool and Char value types [Obsolete]; files still hold them.
                case FieldValue.Bool flag:
                    Record.Type = ObjectDataType.Bool;
                    Record.Value = flag.Value;
                    break;
                case FieldValue.Char letter:
                    Record.Type = ObjectDataType.Char;
                    Record.Value = letter.Value;
                    break;
#pragma warning restore CS0612
                default:
                    throw new ArgumentNullException(nameof(value));
            }
        }
    }

    /// <summary>
    /// Whether the value's type is one of the four a saved file may hold (Int, Real, Unreal, String; not War3Net's
    /// obsolete bool or char, which <see cref="Value"/> shows but <see cref="ObjectFile.ReadExact(Kind, byte[], string?)"/>
    /// refuses).
    /// </summary>
    internal bool HasKnownType => Record.Type is ObjectDataType.Int or ObjectDataType.Real or ObjectDataType.Unreal
        or ObjectDataType.String;

    /// <summary>
    /// A text is stored NUL-terminated as UTF-8, so it can hold neither a NUL nor a broken surrogate pair.
    /// </summary>
    private static void CheckText(string text)
    {
        if (text.Contains('\0'))
        {
            throw new ArgumentException("A text value cannot contain a NUL character.", nameof(Value));
        }

        try
        {
            StrictUtf8.GetByteCount(text);
        }
        catch (EncoderFallbackException e)
        {
            throw new ArgumentException("A text value must be valid Unicode: " + e.Message, nameof(Value), e);
        }
    }

    private static void RequireZero(int value, string name)
    {
        if (value != 0)
        {
            throw new ArgumentException($"{name} must be 0 for a kind without levels.", name);
        }
    }
}

/// <summary>A field's value with its type, as an object file stores it.</summary>
public abstract record FieldValue
{
    private FieldValue()
    {
    }

    /// <summary>A whole number (type 0), such as hit points or a yes/no field.</summary>
    /// <param name="Value">The number.</param>
    public sealed record Int(int Value) : FieldValue;

    /// <summary>A real number (type 1).</summary>
    /// <param name="Value">The number.</param>
    public sealed record Real(float Value) : FieldValue;

    /// <summary>An "unreal" number (type 2), stored as a real is.</summary>
    /// <param name="Value">The number.</param>
    public sealed record Unreal(float Value) : FieldValue;

    /// <summary>
    /// A text (type 3): a string, a list, a model path, or a <c>TRIGSTR_n</c> reference into the .wts file.
    /// </summary>
    /// <param name="Value">The text.</param>
    public sealed record Text(string Value) : FieldValue;

    /// <summary>
    /// A yes/no value (type 4, one byte), which War3Net marks obsolete; no World Editor save has one. <see cref="ObjectFile.Read(Kind, byte[], string?)"/>
    /// keeps it, <see cref="ObjectFile.ReadExact(Kind, byte[], string?)"/> refuses it.
    /// </summary>
    /// <param name="Value">The flag.</param>
    public sealed record Bool(bool Value) : FieldValue;

    /// <summary>
    /// A single character (type 5), which War3Net marks obsolete and reads and writes as UTF-8. <see cref="ObjectFile.Read(Kind, byte[], string?)"/>
    /// keeps it, <see cref="ObjectFile.ReadExact(Kind, byte[], string?)"/> refuses it.
    /// </summary>
    /// <param name="Value">The character.</param>
    public sealed record Char(char Value) : FieldValue;
}
