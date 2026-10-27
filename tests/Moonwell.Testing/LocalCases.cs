namespace Moonwell.Testing
{
    /// <summary>Cases that check Local.For.</summary>
    public static class LocalCases
    {
        /// <summary>The action runs when the player is the one at this machine.</summary>
        public static void RunsForLocalPlayer(FakeWorld world)
        {
            var ran = 0;
            Local.For(Player.Get(0), () => ran++);
            Expect.Equal(1, ran, "runs for the local player");
        }

        /// <summary>The action does not run for another player.</summary>
        public static void SkipsOtherPlayers(FakeWorld world)
        {
            var ran = 0;
            Local.For(Player.Get(1), () => ran++);
            Expect.Equal(0, ran, "runs for another player");
        }

        /// <summary>A read inside Local.For gives the game's value but keeps nothing: once the game has removed the object, synced code reads the same value on the machine that read it locally as on one that never did.</summary>
        public static void ReadsKeepNoLastValue(FakeWorld world)
        {
            var read = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var unread = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var readHero = (Hero)Unit.Of(world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true))!;
            var unreadHero = (Hero)Unit.Of(world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true))!;
            var readItem = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var unreadItem = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            read.PlaceAt(10.0f, 20.0f);
            unread.PlaceAt(10.0f, 20.0f);
            Expect.Near(10.0f, read.X, "the synced x read");
            Expect.Near(10.0f, unread.X, "the other unit's synced x read");

            // The game changes them; only the local machine reads the new values, inside Local.For.
            world.PlaceUnit(read.Handle, 50.0f, 60.0f);
            world.PlaceUnit(unread.Handle, 50.0f, 60.0f);
            world.ScaleUnit(read.Handle, 3.0f);
            world.ScaleUnit(unread.Handle, 3.0f);
            world.SetStrength(readHero.Handle, 30);
            world.SetStrength(unreadHero.Handle, 30);
            world.SetCharges(readItem.Handle, 7);
            world.SetCharges(unreadItem.Handle, 7);
            world.MoveItem(readItem.Handle, 70.0f, 80.0f);
            world.MoveItem(unreadItem.Handle, 70.0f, 80.0f);
            var seenX = 0.0f;
            var seenScale = 0.0f;
            var seenStrength = 0;
            var seenCharges = 0;
            var seenItemX = 0.0f;
            Local.For(Player.Get(0), () =>
            {
                seenX = read.X;
                seenScale = read.Scale;
                seenStrength = readHero.Strength;
                seenCharges = readItem.Charges;
                seenItemX = readItem.X;
            });
            Local.For(Player.Get(1), () =>
            {
                seenX = unread.X;
                seenScale = unread.Scale;
                seenStrength = unreadHero.Strength;
                seenCharges = unreadItem.Charges;
                seenItemX = unreadItem.X;
            });
            Expect.Near(50.0f, seenX, "the local read gives the game's x");
            Expect.Near(3.0f, seenScale, "the local read gives the game's scale");
            Expect.Equal(30, seenStrength, "the local read gives the game's strength");
            Expect.Equal(7, seenCharges, "the local read gives the game's charges");
            Expect.Near(70.0f, seenItemX, "the local read gives the game's item x");

            world.VanishUnit(read.Handle);
            world.VanishUnit(unread.Handle);
            world.VanishUnit(readHero.Handle);
            world.VanishUnit(unreadHero.Handle);
            world.VanishItem(readItem.Handle);
            world.VanishItem(unreadItem.Handle);
            world.Advance(0.3f);
            Expect.True(read.IsGone && unread.IsGone && readHero.IsGone && readItem.IsGone, "the sweep found them gone");

            Expect.Near(unread.X, read.X, "the gone unit's x");
            Expect.Near(unread.Y, read.Y, "the gone unit's y");
            Expect.Near(10.0f, read.X, "the gone unit's x is the last synced read");
            Expect.Near(unread.Scale, read.Scale, "the gone unit's scale");
            Expect.Equal(unreadHero.Strength, readHero.Strength, "the gone hero's strength");
            Expect.Equal(unreadItem.Charges, readItem.Charges, "the gone item's charges");
            Expect.Near(unreadItem.X, readItem.X, "the gone item's x");
        }

        /// <summary>An action that throws inside Local.For leaves no read inside it: the reads after it keep their values again. Local.For reports the error and lets nothing out.</summary>
        public static void ThrowLeavesReadsKept(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            Local.For(Player.Get(0), () => { throw new System.InvalidOperationException("thrown"); });

            unit.PlaceAt(10.0f, 20.0f);
            Expect.Near(10.0f, unit.X, "the x read after the throw");
            world.VanishUnit(unit.Handle);
            world.Advance(0.3f);
            Expect.Near(10.0f, unit.X, "the gone unit's x is the read after the throw");
        }
    }
}
