.class public Lcom/android/server/location/injector/SystemAppOpsHelper;
.super Ljava/lang/Object;
.source "SystemAppOpsHelper.java"

.method public noteOp(ILandroid/location/util/identity/CallerIdentity;)Z
    .registers 14
    .param p1, "op"    # I
    .param p2, "identity"    # Landroid/location/util/identity/CallerIdentity;

    .line 100
    iget-object v0, p0, Lcom/android/server/location/injector/SystemAppOpsHelper;->mAppOps:Landroid/app/AppOpsManager;

    invoke-virtual {v0, p1, p2}, Landroid/app/AppOpsManager;->noteOp(I Landroid/location/util/identity/CallerIdentity;)Z

    move-result v0

    return v0
.end method

.method public noteOpNoThrow(ILandroid/location/util/identity/CallerIdentity;)Z
    .registers 14
    .param p1, "op"    # I
    .param p2, "identity"    # Landroid/location/util/identity/CallerIdentity;

    .line 120
    iget-object v0, p0, Lcom/android/server/location/injector/SystemAppOpsHelper;->mAppOps:Landroid/app/AppOpsManager;

    invoke-virtual {v0, p1, p2}, Landroid/app/AppOpsManager;->noteOpNoThrow(I Landroid/location/util/identity/CallerIdentity;)Z

    move-result v0

    return v0
.end method
