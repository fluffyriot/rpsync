---
layout: default
title: Instagram Setup
---

# Instagram Setup

> **Important**
> Your Instagram account must:
>
> * Be a **Business** or **Creator** account
> * Be **connected to a Facebook Page**
>
> Reference:
>
> * [https://help.instagram.com/1980623166138346/](https://help.instagram.com/1980623166138346/)
> * [https://www.facebook.com/help/instagram/790156881117411](https://www.facebook.com/help/instagram/790156881117411)

## Steps

1. Go to **Meta for Developers** and create an account:
   [https://developers.facebook.com/apps/](https://developers.facebook.com/apps/)

2. Create a new app.

   * Select:

     * *Manage Messaging and content on Instagram*
     * *Manage everything on your Page*

3. In the App Dashboard, navigate to **Settings → Basic**.

4. Copy the **App ID** and **App Secret** into your `.env` file.

5. Open **Facebook Login for Business**.

   * Under **Client OAuth Settings**, add the following to **Valid OAuth Redirect URIs**:

     ```
     https://LOCAL_IP:HTTPS_PORT/auth/facebook/callback
     ```

6. Go to **Use Cases**, edit the **Instagram** use case.

7. Under **API setup with Facebook Login**, add permissions in the **Manage content** section.

8. In **Permissions and Features**, enable:

   * Manage insights
   * Manage comments

9. Open **Graph Explorer**:
   [https://developers.facebook.com/tools/explorer/](https://developers.facebook.com/tools/explorer/)

10. Click **Generate Token**, select your Page and Instagram account.

11. Copy and save the numeric Instagram Page ID displayed.
