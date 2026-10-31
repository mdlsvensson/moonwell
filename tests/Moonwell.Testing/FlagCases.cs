using System;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for HasFlag, which the guide gives for a [Flags] value (MW0010, MW0032): the Lua cases of Moonwell.Objects ran it
    /// in desktop Lua only, so no case the gates play in the game used it. A core enum, an enum CSharp.lua ignores, as it
    /// does those of Moonwell.Objects, whose Lua has their numbers only, and the top bits of a 32-bit integer, which only
    /// the game's Lua has as its integers (Targets goes up to 1 shl 29).
    /// </summary>
    public static class FlagCases
    {
        /// <summary>A core enum: a member, two, one missing, itself, and None, which is in everything.</summary>
        public static void CoreEnum(FakeWorld world)
        {
            var allied = Alliance.Allied;
            Expect.True(allied.HasFlag(Alliance.Passive), "a member of the combination");
            Expect.True(allied.HasFlag(Alliance.Passive | Alliance.SharedXp), "two members");
            Expect.True(!allied.HasFlag(Alliance.SharedVision), "not a member");
            Expect.True(!allied.HasFlag(Alliance.Passive | Alliance.SharedVision), "a member and one that is not");
            Expect.True(allied.HasFlag(allied), "the combination itself");
            Expect.True(allied.HasFlag(Alliance.None), "None in a combination");
            Expect.True(Alliance.None.HasFlag(Alliance.None), "None in None");
            Expect.True(!Alliance.None.HasFlag(Alliance.Passive), "a member in None");
        }

        /// <summary>An enum with no Lua type of its own, held in a field and given through a parameter, as a declared field of Moonwell.Objects is read.</summary>
        public static void IgnoredEnum(FakeWorld world)
        {
            var holder = new Holder { Allowed = Picks.Enemies | Picks.Ground };
            Expect.True(holder.Allowed.HasFlag(Picks.Enemies), "a member, from a field");
            Expect.True(!holder.Allowed.HasFlag(Picks.Air), "not a member, from a field");
            Expect.True(Has(holder.Allowed, Picks.Enemies | Picks.Ground), "both members, through a parameter");
            Expect.True(!Has(holder.Allowed, Picks.Enemies | Picks.Air), "a member and one that is not, through a parameter");
            Expect.True(new Holder().Allowed.HasFlag(Picks.None), "None in an unset field");
            Expect.True(!new Holder().Allowed.HasFlag(Picks.Air), "a member in an unset field");
        }

        /// <summary>The top bits: bit 29, as Targets' last, bit 30, and bit 31, which is the sign of a 32-bit integer.</summary>
        public static void TopBits(FakeWorld world)
        {
            var top = Bits.Last | Bits.High | Bits.Sign;
            Expect.True(top.HasFlag(Bits.Last), "bit 29");
            Expect.True(top.HasFlag(Bits.High), "bit 30");
            Expect.True(top.HasFlag(Bits.Sign), "bit 31");
            Expect.True(top.HasFlag(Bits.High | Bits.Sign), "bits 30 and 31");
            Expect.True(!top.HasFlag(Bits.Low), "bit 0");
            Expect.True(!Bits.High.HasFlag(Bits.Sign), "bit 31 in bit 30");
            Expect.True(!Bits.Sign.HasFlag(Bits.High), "bit 30 in bit 31");
        }

        private static bool Has(Picks value, Picks flag)
        {
            return value.HasFlag(flag);
        }

        private sealed class Holder
        {
            public Picks Allowed;
        }

        /// <summary>Compiled as Moonwell.Objects' choices are.</summary>
        /// @CSharpLua.Ignore
        [Flags]
        private enum Picks
        {
            None = 0,
            Air = 1,
            Enemies = 32,
            Ground = 64,
        }

        [Flags]
        private enum Bits
        {
            Low = 1,
            Last = 1 << 29,
            High = 1 << 30,
            Sign = 1 << 31,
        }
    }
}
