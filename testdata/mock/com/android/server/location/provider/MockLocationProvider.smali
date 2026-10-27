.class public Lcom/android/server/location/provider/MockLocationProvider;
.super Ljava/lang/Object;
.source "MockLocationProvider.java"

.method public setProviderLocation(Landroid/location/Location;)V
    .registers 4
    .param p1, "location"    # Landroid/location/Location;

    .line 55
    new-instance v0, Landroid/location/Location;

    invoke-direct {v0, p1}, Landroid/location/Location;-><init>(Landroid/location/Location;)V

    .line 56
    const/4 v1, 0x1

    invoke-virtual {v0, v1}, Landroid/location/Location;->setIsFromMockProvider(Z)V

    .line 57
    return-void
.end method
