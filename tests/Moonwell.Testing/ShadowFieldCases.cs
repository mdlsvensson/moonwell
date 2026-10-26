using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases with a map's subclass of a core class that has fields of its own named as the core's. CSharp.lua gives a subclass's
    /// field a name of its own only when it sees the base's field of that name, and it does not see a private one, so the core's
    /// fields that a map's class may name are internal: these cases fail in Lua when one of them is private.
    /// </summary>
    public static class ShadowFieldCases
    {
        /// <summary>An owner's count of children and a subclass's own count are two fields.</summary>
        public static void Owner(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new ShadowOwner();
            owner.Adopt(new TestOwned("a", log));
            owner.Adopt(new TestOwned("b", log));
            Expect.Equal(2, owner.OwnedCount, "the owned count");
            Expect.Equal(7, owner.OwnCount, "its own count");
            owner.End();
            Expect.Equal(0, owner.OwnedCount, "the owned count after the end");
            Expect.Equal(7, owner.OwnCount, "its own count after the end");
        }

        /// <summary>A hero's components, abilities and last values and a subclass's own fields of those names are apart.</summary>
        public static void Hero(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H00S"), () => new ShadowHero());
            var handle = world.NewUnit(ObjectTypes.FourCC("H00S"), hero: true);
            var hero = (ShadowHero)Unit.Of(handle)!;

            var tally = hero.Add(new Tally());
            hero.Add(new Tally());
            hero.Remove(tally);
            Expect.Equal(1, hero.ComponentCount, "the components");
            var ability = hero.AbilityOf(ObjectTypes.FourCC("AHhb"));
            Expect.Same(ability, hero.AbilityOf(ObjectTypes.FourCC("AHhb")), "the ability asked for again");

            world.SetStats(handle, 22, 13, 17);
            world.SetStatBonuses(handle, 4, 5, 6);
            world.SetSkillPoints(handle, 2);
            world.SetInventorySize(handle, 6);
            hero.Experience = 700;
            hero.SetLevel(6);
            hero.IsPaused = true;
            hero.Scale = 2.5f;
            hero.FlyHeight = 120.0f;
            hero.MaxLife = 500.0f;
            hero.Life = 450.0f;
            hero.MaxMana = 200.0f;
            hero.Mana = 150.0f;
            hero.FaceNow(45.0f);
            hero.Name = "Renamed";
            hero.ProperName = "Uther";
            Expect.Equal(22, hero.Strength, "strength read");
            Expect.Equal(13, hero.Agility, "agility read");
            Expect.Equal(17, hero.Intelligence, "intelligence read");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus read");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus read");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus read");
            Expect.Equal(2, hero.SkillPoints, "skill points read");
            Expect.Equal(6, hero.InventorySize, "inventory size read");
            Expect.Equal(6, hero.Level, "level read");
            Expect.Near(45.0f, hero.Facing, "facing read");

            world.VanishUnit(handle);
            world.Advance(0.3f);
            Expect.True(hero.IsGone, "the sweep found the hero gone");
            Expect.Near(450.0f, hero.Life, "life");
            Expect.Near(500.0f, hero.MaxLife, "most life");
            Expect.Near(150.0f, hero.Mana, "mana");
            Expect.Near(200.0f, hero.MaxMana, "most mana");
            Expect.Near(45.0f, hero.Facing, "facing");
            Expect.True(hero.IsPaused, "paused");
            Expect.Near(2.5f, hero.Scale, "scale");
            Expect.Near(120.0f, hero.FlyHeight, "fly height");
            Expect.Equal(6, hero.Level, "level");
            Expect.Equal(6, hero.InventorySize, "inventory size");
            Expect.Equal(700, hero.Experience, "experience");
            Expect.Equal(22, hero.Strength, "strength");
            Expect.Equal(13, hero.Agility, "agility");
            Expect.Equal(17, hero.Intelligence, "intelligence");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus");
            Expect.Equal(2, hero.SkillPoints, "skill points");
            var name = "";
            var properName = "";
            Local.For(Player.Get(0), () =>
            {
                name = hero.Name;
                properName = hero.ProperName;
            });
            Expect.Equal("Renamed", name, "name");
            Expect.Equal("Uther", properName, "proper name");
            Expect.Equal("", hero.Changed(), "its own fields that changed");
        }

        /// <summary>An item's last values and a subclass's own fields of those names are apart.</summary>
        public static void Item(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I00S"), () => new ShadowItem());
            var handle = world.NewItem(ObjectTypes.FourCC("I00S"));
            var item = (ShadowItem)Moonwell.Item.Of(handle)!;
            world.MoveItem(handle, 10.0f, 20.0f);
            world.SetCharges(handle, 3);
            Expect.Near(10.0f, item.X, "x read");
            Expect.Near(20.0f, item.Y, "y read");
            Expect.Equal(3, item.Charges, "charges read");

            world.VanishItem(handle);
            world.Advance(0.3f);
            Expect.True(item.IsGone, "the sweep found the item gone");
            Expect.Near(10.0f, item.X, "x");
            Expect.Near(20.0f, item.Y, "y");
            Expect.Equal(3, item.Charges, "charges");
            Expect.Equal("", item.Changed(), "its own fields that changed");
        }
    }
}
