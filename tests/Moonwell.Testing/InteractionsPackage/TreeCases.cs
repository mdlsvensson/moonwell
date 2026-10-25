namespace Moonwell.Testing
{
    /// <summary>Cases for IsTree, which reads the table of tree types the build writes; the fake world says which types are trees.</summary>
    public static class TreeCases
    {
        /// <summary>A destructable of a tree type is a tree, and one of another type is not.</summary>
        public static void Tree(FakeWorld world)
        {
            var fake = new FakeInteractionWorld();
            fake.TreeTypes.Add(ObjectTypes.FourCC("LTlt"));
            Interactions.Install(fake);
            var tree = Destructable.Of(world.NewDestructable(ObjectTypes.FourCC("LTlt")))!;
            var barrel = Destructable.Of(world.NewDestructable(ObjectTypes.FourCC("LTbr")))!;

            Expect.True(tree.IsTree(), "a destructable of a tree type is a tree");
            Expect.True(!barrel.IsTree(), "a destructable of another type is not");
        }
    }
}
