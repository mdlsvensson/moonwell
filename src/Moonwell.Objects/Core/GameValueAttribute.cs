using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// How the game's data spells a member of a choice enum (<see cref="AttackType"/>, <see cref="Targets"/>): the text the
    /// object file holds, then the other spellings the standard data uses (<c>aliv</c> for <c>alive</c>). A value stored as a
    /// number has the number as its text.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Field, AllowMultiple = false, Inherited = false)]
    public sealed class GameValueAttribute : Attribute
    {
        /// <summary>The text the object file holds for the member.</summary>
        public string Text { get; }

        /// <summary>The other spellings the game's data uses for the member; empty when none.</summary>
        public string[] Alternates { get; }

        /// <summary>Marks a member with the game's text and its other spellings.</summary>
        public GameValueAttribute(string text, params string[] alternates)
        {
            Text = text;
            Alternates = alternates;
        }
    }
}
